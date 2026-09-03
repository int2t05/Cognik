// Package session Agent 对话业务逻辑（会话生命周期 + Agent 事件编排）。
//
// 数据存储用独立 SQLite（agent/store），与业务 PostgreSQL 隔离。
// 消息用 parts 数组模型（对标 AI SDK UIMessage.parts）。
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"opsmind/internal/agent"
	"opsmind/internal/agent/store"
	"opsmind/internal/infra/runtime"
	"opsmind/internal/shared/pkg/errcode"

	"gorm.io/gorm"
)

const fallbackAIUnavailable = "当前 AI 服务暂不可用，请提交申告由人工处理"

// =============================================================================
// 流式事件类型
// =============================================================================

// StreamEvent SSE 流式事件（网关交付的事件单元）。
type StreamEvent struct {
	Type     string          `json:"type"`              // reasoning | token | tool_call | tool_result | done | error
	Seq      int             `json:"seq"`               // 网关游标对齐
	Content  string          `json:"content,omitempty"` // token / reasoning / 工具参数 / 工具结果
	ID       string          `json:"id,omitempty"`      // 工具调用 ID
	Label    string          `json:"label,omitempty"`   // 工具名
	Error    string          `json:"error,omitempty"`   // 错误信息
	Metadata *StreamDoneMeta `json:"metadata,omitempty"`
}

// StreamDoneMeta done 事件的元数据。
type StreamDoneMeta struct {
	ThreadID          int64  `json:"thread_id"`
	Question           string `json:"question"`
	Answer             string `json:"answer"`
	CreatedAt          string `json:"created_at"`
	UserMessageID      int64  `json:"user_message_id,omitempty"`
	AssistantMessageID int64  `json:"assistant_message_id,omitempty"`
}

// =============================================================================
// ChatService
// =============================================================================

// ChatService Agent 对话服务。
type ChatService struct {
	store        store.ChatStore          // SQLite 对话数据存储
	agentRunner  *agent.AgentRunner       // 事件生产者（Eino ReAct 循环）
	modelFactory *agent.ChatModelFactory  // AnalyzeFeedback 用 Generate
	gateway      *runtime.Gateway[StreamEvent]
}

// NewChatService 创建 ChatService。
func NewChatService(store store.ChatStore, agentRunner *agent.AgentRunner, modelFactory *agent.ChatModelFactory, gateway *runtime.Gateway[StreamEvent]) *ChatService {
	return &ChatService{
		store:        store,
		agentRunner:  agentRunner,
		modelFactory: modelFactory,
		gateway:      gateway,
	}
}

// CreateThread 创建对话线程。
func (s *ChatService) CreateThread(ctx context.Context, userID int64, title string) (*store.Thread, error) {
	if s.store == nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	thread, err := s.store.CreateThread(ctx, userID, title)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "创建会话失败"}
	}
	return thread, nil
}

// ListThreads 列出用户的对话线程。
func (s *ChatService) ListThreads(ctx context.Context, userID int64) ([]store.Thread, error) {
	if s.store == nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	return s.store.ListThreads(ctx, userID)
}

// GetThreadDetail 获取线程详情（含消息）。
func (s *ChatService) GetThreadDetail(ctx context.Context, threadID, userID int64) (*ThreadDetail, error) {
	if s.store == nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	thread, err := s.store.GetThread(ctx, threadID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
		}
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "加载会话失败"}
	}
	msgs, err := s.store.ListMessages(ctx, threadID)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "加载消息失败"}
	}
	return &ThreadDetail{Thread: *thread, Messages: msgs}, nil
}

// DeleteThread 删除对话线程。
func (s *ChatService) DeleteThread(ctx context.Context, threadID, userID int64) error {
	if s.store == nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	if err := s.store.DeleteThread(ctx, threadID, userID); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "删除会话失败"}
	}
	return nil
}

// UpdateThread 更新线程标题。
func (s *ChatService) UpdateThread(ctx context.Context, threadID, userID int64, title string) error {
	if s.store == nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	thread, err := s.store.GetThread(ctx, threadID, userID)
	if err != nil {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
	}
	thread.Title = title
	return s.store.UpdateThread(ctx, thread)
}

// StreamChat 发送消息并以 SSE 流式返回 Agent 回答。
func (s *ChatService) StreamChat(ctx context.Context, threadID int64, question string, userID int64) ([]StreamEvent, <-chan StreamEvent, func(), error) {
	if strings.TrimSpace(question) == "" {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrParam, Message: "问题不能为空"}
	}
	if s.agentRunner == nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrAIUnavailable, Message: fallbackAIUnavailable}
	}
	if s.store == nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}

	thread, err := s.store.GetThread(ctx, threadID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
		}
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "加载会话失败"}
	}

	// 加载历史消息构建 Eino schema.Message 输入（多轮上下文）
	msgs, msgErr := s.store.ListMessages(ctx, threadID)
	if msgErr != nil {
		slog.Warn("加载历史消息失败，降级为单轮", "thread_id", threadID, "error", msgErr)
	}
	input := s.buildAgentInput(msgs, question)

	// 保存用户消息
	userParts, _ := store.PartsToJSON([]store.MessagePart{
		{Type: store.PartText, Content: question},
	})
	userMsg := &store.Message{
		ThreadID: threadID, Role: "user", Parts: userParts,
		Status: store.MessageStatusCompleted,
	}
	if err := s.store.SaveMessage(ctx, userMsg); err != nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "保存用户消息失败"}
	}

	// 首条消息更新标题
	if thread.Title == "新对话" {
		thread.Title = truncateRunes(question, 50)
		_ = s.store.UpdateThread(ctx, thread)
	}

	// 创建 assistant 占位消息（parts 空数组，streaming 中累积）
	assistantMsg := &store.Message{
		ThreadID: threadID, Role: "assistant", Parts: "[]",
		Status: store.MessageStatusGenerating,
	}
	if err := s.store.SaveMessage(ctx, assistantMsg); err != nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "创建回复占位失败"}
	}

	// detached ctx：客户端断开不停止生成/落库
	gctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	runID := strconv.FormatInt(threadID, 10)
	if err := s.gateway.Start(runID, cancel); err != nil {
		cancel()
		assistantMsg.Status = store.MessageStatusFailed
		_ = s.store.UpdateMessage(context.Background(), assistantMsg)
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrParam, Message: err.Error()}
	}

	// 生产者 goroutine：Agent ReAct 循环 → StreamEvent → gateway.Publish + parts 累积落库
	go s.runAgent(gctx, runID, threadID, userMsg.ID, assistantMsg.ID, question, input)

	replay, ch, unsub, ok := s.gateway.Subscribe(runID, 0)
	if !ok {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "订阅生成失败"}
	}
	return replay, ch, unsub, nil
}

// ThreadDetail 线程详情（含消息）。
type ThreadDetail struct {
	store.Thread
	Messages []store.Message `json:"messages"`
}

// buildAgentInput 从历史消息 + 当前问题构建 Eino schema.Message 输入。
// 从 parts 数组提取 text 部分作为对话内容。
func (s *ChatService) buildAgentInput(msgs []store.Message, question string) []*schema.Message {
	input := make([]*schema.Message, 0, len(msgs)+1)
	for _, m := range msgs {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		parts, err := store.ParseParts(m.Parts)
		if err != nil {
			continue
		}
		// 拼接 text parts 为消息内容
		var content strings.Builder
		for _, p := range parts {
			if p.Type == store.PartText {
				content.WriteString(p.Content)
			}
		}
		if content.Len() == 0 {
			continue
		}
		role := schema.RoleType(m.Role)
		input = append(input, &schema.Message{Role: role, Content: content.String()})
	}
	input = append(input, schema.UserMessage(question))
	return input
}

// runAgent Agent 事件循环 → gateway.Publish + parts 累积落库。
// detached goroutine：客户端断开仍跑完 + 落库。
func (s *ChatService) runAgent(gctx context.Context, runID string, threadID, userMsgID, assistantID int64, question string, input []*schema.Message) {
	defer s.gateway.Finish(runID)

	agentEvents, err := s.agentRunner.Stream(gctx, input)
	if err != nil {
		s.gateway.Publish(runID, StreamEvent{Type: "error", Error: err.Error()})
		s.failMessage(assistantID, err.Error())
		return
	}

	// parts 累积器（落库用）
	var parts []store.MessagePart
	var answer string

	for evt := range agentEvents {
		switch evt.Type {
		case agent.EventReasoning:
			parts = append(parts, store.MessagePart{Type: store.PartReasoning, Content: evt.Content})
		case agent.EventToken:
			answer += evt.Content
			parts = append(parts, store.MessagePart{Type: store.PartText, Content: evt.Content})
		case agent.EventToolCall:
			parts = append(parts, store.MessagePart{
				Type: store.PartToolCall, ID: evt.ID, Label: evt.Label,
				Content: evt.Content, Status: "running",
			})
		case agent.EventToolResult:
			parts = append(parts, store.MessagePart{
				Type: store.PartToolResult, ID: evt.ID, Label: evt.Label,
				Content: evt.Content, Status: "done",
			})
		case agent.EventDone:
			// 落库（parts 数组 + completed 状态）
			partsJSON, _ := store.PartsToJSON(parts)
			_ = s.store.UpdateMessage(context.Background(), &store.Message{
				ID:     assistantID,
				Parts:  partsJSON,
				Status: store.MessageStatusCompleted,
			})
			now := time.Now().Format("2006-01-02 15:04:05")
			s.gateway.Publish(runID, StreamEvent{Type: "done", Metadata: &StreamDoneMeta{
				Answer:             answer,
				ThreadID:           threadID,
				Question:           question,
				AssistantMessageID: assistantID,
				UserMessageID:      userMsgID,
				CreatedAt:          now,
			}})
			continue
		case agent.EventError:
			s.failMessage(assistantID, evt.Error)
		}
		// 透传事件（token/reasoning/tool_call/tool_result/error），seq 由 gateway.Publish 的 setSeq 对齐
		s.gateway.Publish(runID, StreamEvent{
			Type:    evt.Type,
			Content: evt.Content,
			ID:      evt.ID,
			Label:   evt.Label,
			Error:   evt.Error,
		})
	}

	// 超时/取消：保留部分回答（不删除），标记 cancelled
	if gctx.Err() != nil {
		partsJSON, _ := store.PartsToJSON(parts)
		_ = s.store.UpdateMessage(context.Background(), &store.Message{
			ID:     assistantID,
			Parts:  partsJSON,
			Status: store.MessageStatusCancelled,
		})
		s.gateway.Publish(runID, StreamEvent{Type: "error", Error: "生成已停止"})
	}
}

// failMessage 标记消息失败。
func (s *ChatService) failMessage(msgID int64, errMsg string) {
	_ = s.store.UpdateMessage(context.Background(), &store.Message{
		ID:     msgID,
		Status: store.MessageStatusFailed,
		Error:  errMsg,
	})
}

// ResumeStream 续传进行中的生成。
func (s *ChatService) ResumeStream(ctx context.Context, threadID, userID int64, since int) ([]StreamEvent, <-chan StreamEvent, func(), error) {
	if _, err := s.store.GetThread(ctx, threadID, userID); err != nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
	}
	replay, ch, unsub, ok := s.gateway.Subscribe(strconv.FormatInt(threadID, 10), since)
	if !ok {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "无进行中的生成"}
	}
	return replay, ch, unsub, nil
}

// CancelGeneration 取消生成。
func (s *ChatService) CancelGeneration(ctx context.Context, threadID, userID int64) error {
	if _, err := s.store.GetThread(ctx, threadID, userID); err != nil {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
	}
	if !s.gateway.Cancel(strconv.FormatInt(threadID, 10)) {
		return errcode.AppError{Code: errcode.ErrParam, Message: "无进行中的生成"}
	}
	return nil
}

// CleanupStale 清理启动时残留的 generating 状态消息。
func (s *ChatService) CleanupStale(ctx context.Context) error {
	_, err := s.store.CleanupStale(ctx)
	return err
}

// AnalyzeFeedback 分析反馈数据（用 Eino ChatModel Generate）。
func (s *ChatService) AnalyzeFeedback(ctx context.Context, samples []FeedbackSample) (string, error) {
	if s.modelFactory == nil || s.modelFactory.GetModel() == nil {
		return "", errcode.AppError{Code: errcode.ErrAIUnavailable, Message: "LLM 服务未初始化"}
	}
	if len(samples) == 0 {
		return "{\"message\":\"暂无反馈数据可供分析。\"}", nil
	}

	var helpful, unhelpful strings.Builder
	helpfulCount, unhelpfulCount := 0, 0
	for _, s := range samples {
		question := truncateRunes(s.Question, 200)
		answer := truncateRunes(s.Answer, 300)
		if s.Feedback == 1 {
			helpfulCount++
			fmt.Fprintf(&helpful, "- Q: %s\n  A: %s\n", question, answer)
		} else {
			unhelpfulCount++
			fmt.Fprintf(&unhelpful, "- Q: %s\n  A: %s\n", question, answer)
		}
	}

	prompt := fmt.Sprintf(`你是运维知识库的质量分析师。请根据以下用户反馈数据分析知识库的优缺点。

## 用户标记为"有帮助"的回答（共 %d 条）：
%s

## 用户标记为"无帮助"的回答（共 %d 条）：
%s

请用 JSON 格式输出分析结果（只输出 JSON，不要其他内容）：
{
  "strong_areas": ["方面1", "方面2"],
  "weak_areas": ["方面1", "方面2"],
  "suggestions": ["建议1", "建议2"],
  "summary": "一句话总结（30字以内）"
}`, helpfulCount, helpful.String(), unhelpfulCount, unhelpful.String())

	msgs := []*schema.Message{
		{Role: schema.System, Content: "你是运维知识库质量分析师。根据用户反馈数据，识别知识盲区和改进方向。只输出 JSON，不要任何解释。"},
		{Role: schema.User, Content: prompt},
	}
	resp, err := s.modelFactory.GetModel().Generate(ctx, msgs)
	if err != nil {
		return "", fmt.Errorf("LLM 分析调用失败: %w", err)
	}
	return resp.Content, nil
}

// FeedbackSample 反馈样本（供 AnalyzeFeedback 用）。
type FeedbackSample struct {
	Question string
	Answer   string
	Feedback int16
}

// truncateRunes 截断字符串到指定 rune 数。
func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

// confidenceLevel 置信度分级（历史会话展示用）。
func confidenceLevel(raw float64) string {
	const defaultLowT, defaultHighT = 0.40, 0.70
	if raw >= defaultHighT {
		return "high"
	}
	if raw >= defaultLowT {
		return "medium"
	}
	return "low"
}

// 引用 json 防止未使用（AnalyzeFeedback 后续可能用 JSON 解析响应）
var _ = json.Marshal
