// Package session Agent 对话业务逻辑（会话生命周期 + Agent 事件编排）。
//
// 数据存储用独立 SQLite（agent/store），与业务 PostgreSQL 隔离。
// 消息用 parts 数组模型（对标 AI SDK UIMessage.parts）。
package session

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"cognos/internal/agent"
	"cognos/internal/agent/store"
	"cognos/internal/infra/runtime"
	"cognos/internal/shared/pkg/errcode"

	"gorm.io/gorm"
)

const fallbackAIUnavailable = "当前 AI 服务暂不可用，请提交申告由人工处理"

// StreamEvent SSE 流式事件（网关交付的事件单元）。
type StreamEvent struct {
	Type     string          `json:"type"`              // reasoning | token | tool_call | tool_result | done | error
	Seq      int             `json:"seq"`               // 网关游标对齐
	Content  string          `json:"content,omitempty"` // token / reasoning / 工具参数 / 工具结果
	ID       string          `json:"id,omitempty"`      // 工具调用 ID
	Label    string          `json:"label,omitempty"`   // 工具名
	TaskID   string          `json:"task_id,omitempty"` // 子 Agent 来源 task ID（空=主 Agent；非空=子 Agent 事件）
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

// ChatService Agent 对话服务。
type ChatService struct {
	store        store.ChatStore           // SQLite 对话数据存储
	agentRunner  *agent.AgentRunner        // 事件生产者（Eino ReAct 循环）
	gateway      *runtime.Gateway[StreamEvent]
	onSessionEnd func(ctx context.Context, threadID int64) error // 会话结束钩子（记忆提取，best-effort）
}

// ChatServiceOption 函数选项模式。
type ChatServiceOption func(*ChatService)

// WithSessionEndHook 设置会话结束钩子（删除会话前触发记忆提取）。
func WithSessionEndHook(fn func(ctx context.Context, threadID int64) error) ChatServiceOption {
	return func(s *ChatService) { s.onSessionEnd = fn }
}

// NewChatService 创建 ChatService。
func NewChatService(store store.ChatStore, agentRunner *agent.AgentRunner, gateway *runtime.Gateway[StreamEvent], opts ...ChatServiceOption) *ChatService {
	s := &ChatService{
		store:       store,
		agentRunner: agentRunner,
		gateway:     gateway,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
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

// DeleteThread 删除对话线程。会话结束前触发记忆提取（best-effort，失败不阻塞删除）。
func (s *ChatService) DeleteThread(ctx context.Context, threadID, userID int64) error {
	if s.store == nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	// 会话结束提取：扫描会话记忆 → LLM 提取 → 写入 global（best-effort）
	if s.onSessionEnd != nil {
		if err := s.onSessionEnd(ctx, threadID); err != nil {
			slog.Warn("会话记忆提取失败", "thread_id", threadID, "error", err)
		}
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

	// detached ctx：客户端断开不停止生成/落库（10 分钟上限，容纳 deep_research pipeline 多轮搜索）
	gctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
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
	// taskID → tool_call part 下标（子 Agent 事件归入对应 dispatch_subagent 卡片）。
	taskParts := make(map[string]int)
	// 增量写入计数器：每 persistEvery 个事件落库一次（status=generating），中断可恢复。
	persistEvery := 10
	eventCount := 0
	// persistParts 增量写入 parts 到 DB（status=generating），保证中断可恢复。
	persistParts := func(status string) {
		partsJSON, _ := store.PartsToJSON(parts)
		_ = s.store.UpdateMessage(context.Background(), &store.Message{
			ID:     assistantID,
			Parts:  partsJSON,
			Status: status,
		})
	}

	// findOrInitTaskPart 按 taskID 找/建 tool_call part，返回下标（子 Agent 事件归入此卡片）。
	// ack tool_result 带 ID=tool_use_id + TaskID：映射到已有的 dispatch_subagent part（按 ID 匹配）。
	// 后续子 Agent 事件只带 TaskID（无 ID）：复用已建立的映射，没有则新建卡片。
	findOrInitTaskPart := func(taskID, toolCallID string) int {
		if idx, ok := taskParts[taskID]; ok {
			return idx
		}
		// ack 阶段：按 tool_use_id 找已有的 dispatch_subagent part。
		if toolCallID != "" {
			for i := range parts {
				if parts[i].Type == store.PartToolCall && parts[i].ID == toolCallID {
					taskParts[taskID] = i
					return i
				}
			}
		}
		// 首次见到 taskID 但无对应 part（异常）：新建承载卡片。
		parts = append(parts, store.MessagePart{
			Type: store.PartToolCall, ID: taskID, Label: "dispatch_subagent",
			Content: "", Status: "running",
		})
		idx := len(parts) - 1
		taskParts[taskID] = idx
		return idx
	}

	for evt := range agentEvents {
		// 带 TaskID 的事件来自子 Agent → 归入对应 dispatch_subagent 卡片，不混入主 Agent 文本。
		if evt.TaskID != "" {
			idx := findOrInitTaskPart(evt.TaskID, evt.ID)
			switch evt.Type {
			case agent.EventReasoning, agent.EventToken:
				parts[idx].Content += evt.Content
			case agent.EventToolCall:
				parts[idx].Content += "\n[tool_call] " + evt.Label + ": " + evt.Content
			case agent.EventToolResult:
				// ack tool_result（id=tool_use_id ≠ task_id）：追加内容，保持 running。
				// task_completion tool_result（id=task_id）：追加最终结果 + 标记 done。
				if evt.ID == evt.TaskID {
					parts[idx].Status = "done"
					parts[idx].Content += "\n--- result ---\n" + evt.Content
				} else if evt.ID != "" {
					parts[idx].Content += "\n" + evt.Content
				}
			}
			s.gateway.Publish(runID, StreamEvent{
				Type: evt.Type, Content: evt.Content, ID: evt.ID,
				Label: evt.Label, TaskID: evt.TaskID, Error: evt.Error,
			})
			// 子 Agent 事件增量写：结构事件立即写，其他去抖。
			eventCount++
			if evt.Type == agent.EventToolCall || evt.Type == agent.EventToolResult || eventCount%persistEvery == 0 {
				persistParts(store.MessageStatusGenerating)
			}
			continue
		}
		switch evt.Type {
		case agent.EventReasoning:
			// 合并到最后一个 reasoning part（若最后是 reasoning 则追加，否则新建）
			if n := len(parts); n > 0 && parts[n-1].Type == store.PartReasoning {
				parts[n-1].Content += evt.Content
			} else {
				parts = append(parts, store.MessagePart{Type: store.PartReasoning, Content: evt.Content})
			}
		case agent.EventToken:
			answer += evt.Content
			// 合并到最后一个 text part
			if n := len(parts); n > 0 && parts[n-1].Type == store.PartText {
				parts[n-1].Content += evt.Content
			} else {
				parts = append(parts, store.MessagePart{Type: store.PartText, Content: evt.Content})
			}
		case agent.EventToolCall:
			// pipeline 中间步骤（无 ID，有 Label）→ 追加到最后一个 running tool_call
			if evt.ID == "" && evt.Label != "" {
				if n := len(parts); n > 0 && parts[n-1].Type == store.PartToolCall && parts[n-1].Status == "running" {
					parts[n-1].Content += "\n" + evt.Content
				}
			} else {
				// 正常工具调用（有 ID）→ 按 ID 匹配或新建
				found := false
				if evt.ID != "" {
					for i := range parts {
						if parts[i].Type == store.PartToolCall && parts[i].ID == evt.ID {
							parts[i].Content += evt.Content
							found = true
							break
						}
					}
				}
				if !found {
					for i := len(parts) - 1; i >= 0; i-- {
						if parts[i].Type == store.PartToolCall && parts[i].Status == "running" && parts[i].Label != "" {
							if !strings.HasSuffix(strings.TrimRight(parts[i].Content, " \t\r\n"), "}") {
								parts[i].Content += evt.Content
								found = true
							}
							break
						}
					}
				}
				if !found {
					parts = append(parts, store.MessagePart{
						Type: store.PartToolCall, ID: evt.ID, Label: evt.Label,
						Content: evt.Content, Status: "running",
					})
				}
			}
		case agent.EventToolResult:
			// 配对到同 ID 的 tool_call part，更新 status=done + result
			found := false
			for i := range parts {
				if parts[i].Type == store.PartToolCall && parts[i].ID == evt.ID {
					parts[i].Status = "done"
					parts[i].Content += "\n--- result ---\n" + evt.Content
					found = true
					break
				}
			}
			if !found {
				// 无对应 tool_call（异常），单独创建
				parts = append(parts, store.MessagePart{
					Type: store.PartToolResult, ID: evt.ID, Label: evt.Label,
					Content: evt.Content, Status: "done",
				})
			}
		case agent.EventDone:
			// 最终落库（parts + completed 状态）
			persistParts(store.MessageStatusCompleted)
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
		// 增量写入：tool_call/tool_result 立即写（结构事件重要），其他事件去抖写。
		eventCount++
		if evt.Type == agent.EventToolCall || evt.Type == agent.EventToolResult || eventCount%persistEvery == 0 {
			persistParts(store.MessageStatusGenerating)
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
		persistParts(store.MessageStatusCancelled)
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

// truncateRunes 截断字符串到指定 rune 数。
func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}
