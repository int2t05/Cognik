// Package session 问答会话业务逻辑（会话生命周期 + RAG/LLM 编排）。
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"strconv"
	"time"

	"github.com/cloudwego/eino/schema"
	"opsmind/internal/agent"
	"opsmind/internal/domain/system/audit"
	"opsmind/internal/infra/runtime"
	"opsmind/internal/shared/dto/request"
	respDto "opsmind/internal/shared/dto/response"
	"opsmind/internal/shared/model"
	"opsmind/internal/shared/pkg/errcode"

	"gorm.io/gorm"
)

const (
	fallbackAIUnavailable = "当前 AI 服务暂不可用，请提交申告由人工处理"
)

// =============================================================================
// 消费者接口
// =============================================================================

type chatKnowledgeRepo interface {
	FindKBByID(ctx context.Context, id int64) (*model.KnowledgeBase, error)
}

type chatSessionRepo interface {
	Create(ctx context.Context, session *model.ChatSession) error
	CreateBatch(ctx context.Context, messages []model.ChatMessage) error
	CreateMessage(ctx context.Context, m *model.ChatMessage) error
	UpdateMessage(ctx context.Context, m *model.ChatMessage) error
	DeleteMessage(ctx context.Context, id int64) error
	CleanFailedMessages(ctx context.Context, sessionID int64) (int64, error)
	MarkGeneratingFailed(ctx context.Context) (int64, error)
	FindByID(ctx context.Context, id int64) (*model.ChatSession, error)
	FindMessageByID(ctx context.Context, messageID, sessionID int64) (*model.ChatMessage, error)
	FindMessagesBySession(ctx context.Context, sessionID int64) ([]model.ChatMessage, error)
	UpdateFeedback(ctx context.Context, id int64, feedback int16) error
	UpdateMessageFeedback(ctx context.Context, messageID int64, feedback int16) error
	FindFeedbackSamples(ctx context.Context, limitDays int) ([]model.FeedbackSample, error)
	UpdateSession(ctx context.Context, session *model.ChatSession) error
	UpdateSessionMeta(ctx context.Context, sessionID int64, question string, kbID int64) error
	ListByUser(ctx context.Context, userID int64, page, pageSize int) ([]model.ChatSession, int64, error)
	DeleteSession(ctx context.Context, id, userID int64) error
	CountMessagesBySession(ctx context.Context, sessionID int64) (int64, error)
	CountMessagesBySessions(ctx context.Context, sessionIDs []int64) (map[int64]int64, error)
}

type ragConfigReader interface {
	GetInt(ctx context.Context, key string) (int, bool)
	GetFloat(ctx context.Context, key string) (float64, bool)
	GetBool(ctx context.Context, key string) (bool, bool)
}

// =============================================================================
// 流式事件类型
// =============================================================================

type StreamEvent struct {
	Type     string          `json:"type"`
	Seq      int             `json:"seq"`
	Content  string          `json:"content,omitempty"`
	ID       string          `json:"id,omitempty"`
	Label    string          `json:"label,omitempty"`
	Error    string          `json:"error,omitempty"`
	Metadata *StreamDoneMeta `json:"metadata,omitempty"`
}

// StreamDoneMeta done 事件的元数据（Agent 完成回答后的落库标识）。
type StreamDoneMeta struct {
	SessionID          int64  `json:"session_id"`
	Question           string `json:"question"`
	Answer             string `json:"answer"`
	CreatedAt          string `json:"created_at"`
	UserMessageID      int64  `json:"user_message_id,omitempty"`
	AssistantMessageID int64 `json:"assistant_message_id,omitempty"`
}

// =============================================================================
// ChatService
// =============================================================================

type ChatService struct {
	configReader  ragConfigReader
	knowledgeRepo chatKnowledgeRepo
	chatRepo      chatSessionRepo
	agentRunner   *agent.AgentRunner     // 事件生产者（Eino ReAct 循环）
	modelFactory  *agent.ChatModelFactory // AnalyzeFeedback 用 Generate
	auditRepo     audit.AuditWriter
	gateway       *runtime.Gateway[StreamEvent]
}

// NewChatService 创建 ChatService。
// 生产者：agentRunner 事件循环 + modelFactory Generate。
func NewChatService(knowledgeRepo chatKnowledgeRepo, chatRepo chatSessionRepo, agentRunner *agent.AgentRunner, modelFactory *agent.ChatModelFactory, configReader ragConfigReader, auditRepo audit.AuditWriter, gateway *runtime.Gateway[StreamEvent]) *ChatService {
	return &ChatService{
		knowledgeRepo: knowledgeRepo,
		chatRepo:      chatRepo,
		agentRunner:   agentRunner,
		modelFactory:  modelFactory,
		configReader:  configReader,
		auditRepo:     auditRepo,
		gateway:       gateway,
	}
}

func (s *ChatService) CreateSession(ctx context.Context, req request.CreateSessionRequest, userID int64) (*model.ChatSession, error) {
	if s.knowledgeRepo != nil {
		if _, err := s.knowledgeRepo.FindKBByID(ctx, req.KBID); err != nil {
			return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "知识库不存在"}
		}
	}
	if s.chatRepo == nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "新会话"
	}

	session := &model.ChatSession{
		UserID:   userID,
		KBID:     req.KBID,
		Question: title,
	}
	if err := s.chatRepo.Create(ctx, session); err != nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "创建会话失败"}
	}

	return session, nil
}

func (s *ChatService) StreamChat(ctx context.Context, sessionID int64, question string, userID int64) ([]StreamEvent, <-chan StreamEvent, func(), error) {
	if strings.TrimSpace(question) == "" {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrParam, Message: "问题不能为空"}
	}
	if s.agentRunner == nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrAIUnavailable, Message: fallbackAIUnavailable}
	}
	if s.chatRepo == nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}

	session, err := s.chatRepo.FindByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
		}
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "加载会话失败，请稍后重试"}
	}
	if session.UserID != userID {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrForbidden, Message: "无权访问该会话"}
	}

	// 加载历史消息构建 Eino schema.Message 输入（多轮上下文）
	msgs, msgErr := s.chatRepo.FindMessagesBySession(ctx, sessionID)
	if msgErr != nil {
		slog.Warn("加载会话历史消息失败，多轮上下文降级为单轮", "session_id", sessionID, "error", msgErr)
	}
	input := s.buildAgentInput(msgs, question)

	userMsg := &model.ChatMessage{SessionID: sessionID, Role: "user", Content: question, Status: model.MessageStatusCompleted}
	if err := s.chatRepo.CreateMessage(ctx, userMsg); err != nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "保存用户消息失败"}
	}
	if session.Question == "新会话" || session.Question == "" {
		_ = s.chatRepo.UpdateSessionMeta(ctx, sessionID, question, 0)
	}

	assistant := &model.ChatMessage{SessionID: sessionID, Role: "assistant", Content: "", Status: model.MessageStatusGenerating}
	if err := s.chatRepo.CreateMessage(ctx, assistant); err != nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "创建回复占位失败"}
	}

	// detached ctx：客户端断开不停止生成/落库
	genTimeout := 120 * time.Second
	if s.readBool("ai.enable_thinking", false) {
		genTimeout = 300 * time.Second
	}
	gctx, cancel := context.WithTimeout(context.Background(), genTimeout)
	runID := strconv.FormatInt(sessionID, 10)
	if err := s.gateway.Start(runID, cancel); err != nil {
		cancel()
		assistant.Status = model.MessageStatusFailed
		_ = s.chatRepo.UpdateMessage(context.Background(), assistant)
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrParam, Message: err.Error()}
	}

	// 生产者 goroutine：Agent ReAct 循环 → AgentEvent → StreamEvent → gateway.Publish
	go s.runAgent(gctx, runID, sessionID, userMsg.ID, assistant.ID, question, input)

	replay, ch, unsub, ok := s.gateway.Subscribe(runID, 0)
	if !ok {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "订阅生成失败"}
	}
	return replay, ch, unsub, nil
}

// buildAgentInput 从历史消息 + 当前问题构建 Eino schema.Message 输入。
func (s *ChatService) buildAgentInput(msgs []model.ChatMessage, question string) []*schema.Message {
	input := make([]*schema.Message, 0, len(msgs)+1)
	for _, m := range msgs {
		role := schema.RoleType(m.Role)
		if role != schema.User && role != schema.Assistant {
			continue
		}
		input = append(input, &schema.Message{Role: role, Content: m.Content})
	}
	input = append(input, schema.UserMessage(question))
	return input
}

// runAgent Agent 事件循环 → gateway.Publish。
// detached goroutine：客户端断开仍跑完 + 落库。
func (s *ChatService) runAgent(gctx context.Context, runID string, sessionID, userMsgID, assistantID int64, question string, input []*schema.Message) {
	defer s.gateway.Finish(runID)

	agentEvents, err := s.agentRunner.Stream(gctx, input)
	if err != nil {
		s.gateway.Publish(runID, StreamEvent{Type: "error", Error: err.Error()})
		s.failAssistant(assistantID)
		return
	}

	var answer string
	for evt := range agentEvents {
		switch evt.Type {
		case agent.EventToken:
			answer += evt.Content
		case agent.EventDone:
			// 落库（无 sources/confidence/pipeline — Agent 无 RAG）
			now := time.Now().Format("2006-01-02 15:04:05")
			_ = s.chatRepo.UpdateSession(context.Background(), &model.ChatSession{
				ID: sessionID, Answer: answer,
			})
			_ = s.chatRepo.UpdateMessage(context.Background(), &model.ChatMessage{
				ID: assistantID, Content: answer, Status: model.MessageStatusCompleted,
			})
			s.gateway.Publish(runID, StreamEvent{Type: "done", Metadata: &StreamDoneMeta{
				Answer:             answer,
				SessionID:          sessionID,
				Question:           question,
				AssistantMessageID: assistantID,
				UserMessageID:      userMsgID,
				CreatedAt:          now,
			}})
			continue
		case agent.EventError:
			s.failAssistant(assistantID)
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

	if gctx.Err() != nil {
		if errors.Is(gctx.Err(), context.DeadlineExceeded) {
			_ = s.chatRepo.UpdateMessage(context.Background(), &model.ChatMessage{
				ID: assistantID, Content: answer, Status: model.MessageStatusFailed,
			})
		} else {
			_ = s.chatRepo.DeleteMessage(context.Background(), userMsgID)
			_ = s.chatRepo.DeleteMessage(context.Background(), assistantID)
		}
		s.gateway.Publish(runID, StreamEvent{Type: "error", Error: "生成已停止"})
	}
}

func (s *ChatService) failAssistant(msgID int64) {
	_ = s.chatRepo.UpdateMessage(context.Background(), &model.ChatMessage{ID: msgID, Status: model.MessageStatusFailed})
}

func (s *ChatService) ResumeStream(ctx context.Context, sessionID, userID int64, since int) ([]StreamEvent, <-chan StreamEvent, func(), error) {
	session, err := s.chatRepo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
	}
	if session.UserID != userID {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrForbidden, Message: "无权访问该会话"}
	}
	replay, ch, unsub, ok := s.gateway.Subscribe(strconv.FormatInt(sessionID, 10), since)
	if !ok {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "无进行中的生成"}
	}
	return replay, ch, unsub, nil
}

func (s *ChatService) CancelGeneration(ctx context.Context, sessionID, userID int64) error {
	session, err := s.chatRepo.FindByID(ctx, sessionID)
	if err != nil {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
	}
	if session.UserID != userID {
		return errcode.AppError{Code: errcode.ErrForbidden, Message: "无权访问该会话"}
	}
	if !s.gateway.Cancel(strconv.FormatInt(sessionID, 10)) {
		return errcode.AppError{Code: errcode.ErrParam, Message: "无进行中的生成"}
	}
	return nil
}

func (s *ChatService) CleanupStaleGenerating(ctx context.Context) error {
	_, err := s.chatRepo.MarkGeneratingFailed(ctx)
	return err
}

func (s *ChatService) SubmitMessageFeedback(ctx context.Context, messageID, sessionID, userID int64, feedback int16) error {
	if s.chatRepo == nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	if feedback < 0 || feedback > 2 {
		return errcode.AppError{Code: errcode.ErrParam, Message: "反馈值无效，请输入 0（取消）/1（有帮助）/2（无帮助）"}
	}

	session, err := s.chatRepo.FindByID(ctx, sessionID)
	if err != nil {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
	}
	if session.UserID != userID {
		return errcode.AppError{Code: errcode.ErrForbidden, Message: "无权操作该会话"}
	}

	msg, err := s.chatRepo.FindMessageByID(ctx, messageID, sessionID)
	if err != nil {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "消息不存在"}
	}
	if msg.Role != "assistant" {
		return errcode.AppError{Code: errcode.ErrParam, Message: "仅可对 AI 回答进行反馈"}
	}

	if err := s.chatRepo.UpdateMessageFeedback(ctx, messageID, feedback); err != nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "反馈提交失败"}
	}

	s.writeFeedbackAudit(ctx, userID, sessionID, messageID, feedback, "explicit")
	return nil
}

func (s *ChatService) MarkLastAssistantUnhelpful(ctx context.Context, sessionID int64) error {
	if s.chatRepo == nil {
		return nil
	}

	msgs, err := s.chatRepo.FindMessagesBySession(ctx, sessionID)
	if err != nil {
		return err
	}

	var lastAssistant *model.ChatMessage
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			lastAssistant = &msgs[i]
			break
		}
	}
	if lastAssistant == nil {
		return nil
	}
	if lastAssistant.Feedback == 1 {
		return nil
	}

	if err := s.chatRepo.UpdateMessageFeedback(ctx, lastAssistant.ID, 2); err != nil {
		return err
	}

	s.writeFeedbackAudit(ctx, 0, sessionID, lastAssistant.ID, 2, "implicit")
	return nil
}

func (s *ChatService) writeFeedbackAudit(ctx context.Context, userID, sessionID, messageID int64, feedback int16, source string) {
	if s.auditRepo == nil {
		return
	}
	detail, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"message_id": messageID,
		"feedback":   feedback,
		"source":     source,
	})
	if err := s.auditRepo.Write(ctx, userID, "chat.feedback", "chat_message", messageID, string(detail)); err != nil {
		slog.Warn("反馈审计日志写入失败", "message_id", messageID, "error", err)
	}
}

func (s *ChatService) SubmitFeedback(ctx context.Context, sessionID int64, userID int64, feedback int16) error {
	if s.chatRepo == nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	if feedback < 0 || feedback > 2 {
		return errcode.AppError{Code: errcode.ErrParam, Message: "反馈值无效，请输入 0（未反馈）/1（已解决）/2（未解决）"}
	}
	if feedback == 0 {
		return errcode.AppError{Code: errcode.ErrParam, Message: "反馈值无效，请输入 1（已解决）或 2（未解决）"}
	}
	session, err := s.chatRepo.FindByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
		}
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "加载会话失败，请稍后重试"}
	}
	if session.UserID != userID {
		return errcode.AppError{Code: errcode.ErrForbidden, Message: "无权操作该会话"}
	}
	return s.chatRepo.UpdateFeedback(ctx, sessionID, feedback)
}

func (s *ChatService) GetChatDetail(ctx context.Context, sessionID int64, userID int64) (*respDto.ChatSessionResponse, error) {
	if s.chatRepo == nil {
		return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	session, err := s.chatRepo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
	}
	if session.UserID != userID {
		return nil, errcode.AppError{Code: errcode.ErrForbidden, Message: "无权查看该会话"}
	}

	if cleaned, err := s.chatRepo.CleanFailedMessages(ctx, sessionID); err != nil {
		slog.Warn("清理失败消息对失败", "session_id", sessionID, "error", err)
	} else if cleaned > 0 {
		slog.Info("已清理失败消息对", "session_id", sessionID, "pairs", cleaned/2)
	}

	var sources []respDto.SourceItem
	if len(session.Sources) > 0 {
		if err := json.Unmarshal(session.Sources, &sources); err != nil {
			slog.Warn("解析会话 Sources JSON 失败", "session_id", sessionID, "error", err)
		}
	}

	var messages []respDto.MessageItem
	if msgs, msgErr := s.chatRepo.FindMessagesBySession(ctx, sessionID); msgErr == nil {
		for _, m := range msgs {
			var msgSources []respDto.SourceItem
			if len(m.Sources) > 0 {
				if err := json.Unmarshal(m.Sources, &msgSources); err != nil {
					slog.Warn("解析消息 Sources JSON 失败", "message_id", m.ID, "error", err)
				}
			}
			messages = append(messages, respDto.MessageItem{
				ID:              m.ID,
				Role:            m.Role,
				Content:         m.Content,
				Sources:         msgSources,
				Confidence:      m.ConfidenceRaw,
				ConfidenceRaw:   m.ConfidenceRaw,
				ConfidenceLevel: confidenceLevel(m.ConfidenceRaw),
				Feedback:        m.Feedback,
				Status:          m.Status,
				CreatedAt:       m.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
	}

	return &respDto.ChatSessionResponse{
		SessionID:       session.ID,
		Question:        session.Question,
		Answer:          session.Answer,
		Sources:         sources,
		Confidence:      session.Confidence,
		ConfidenceRaw:   session.Confidence,
		ConfidenceLevel: confidenceLevel(session.Confidence),
		CanSubmitTicket: confidenceLevel(session.Confidence) != "high",
		DurationMS:      session.DurationMs,
		Feedback:        session.Feedback,
		CreatedAt:       session.CreatedAt.Format("2006-01-02 15:04:05"),
		Messages:        messages,
	}, nil
}

func (s *ChatService) ListSessions(ctx context.Context, userID int64, page, pageSize int) ([]respDto.SessionListItem, int64, error) {
	if s.chatRepo == nil {
		return nil, 0, errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	sessions, total, err := s.chatRepo.ListByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("查询会话列表失败: %w", err)
	}

	sessionIDs := make([]int64, len(sessions))
	for i, sess := range sessions {
		sessionIDs[i] = sess.ID
	}
	msgCounts, countErr := s.chatRepo.CountMessagesBySessions(ctx, sessionIDs)
	if countErr != nil {
		slog.Warn("批量获取会话消息数失败", "error", countErr)
		msgCounts = map[int64]int64{}
	}

	items := make([]respDto.SessionListItem, 0, len(sessions))
	for _, sess := range sessions {
		lastAnswer := truncateRunes(sess.Answer, 100)
		items = append(items, respDto.SessionListItem{
			ID:           sess.ID,
			KBID:         sess.KBID,
			Question:     sess.Question,
			LastAnswer:   lastAnswer,
			MessageCount: msgCounts[sess.ID],
			CreatedAt:    sess.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:    sess.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return items, total, nil
}

func (s *ChatService) DeleteSession(ctx context.Context, sessionID, userID int64) error {
	if s.chatRepo == nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	session, err := s.chatRepo.FindByID(ctx, sessionID)
	if err != nil {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
	}
	if session.UserID != userID {
		return errcode.AppError{Code: errcode.ErrForbidden, Message: "无权删除该会话"}
	}
	return s.chatRepo.DeleteSession(ctx, sessionID, userID)
}

func (s *ChatService) UpdateSessionMeta(ctx context.Context, sessionID, userID int64, question string, kbID int64) error {
	if s.chatRepo == nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	session, err := s.chatRepo.FindByID(ctx, sessionID)
	if err != nil {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
	}
	if session.UserID != userID {
		return errcode.AppError{Code: errcode.ErrForbidden, Message: "无权修改该会话"}
	}
	return s.chatRepo.UpdateSessionMeta(ctx, sessionID, question, kbID)
}

func (s *ChatService) readBool(key string, fallback bool) bool {
	if s.configReader == nil {
		return fallback
	}
	if v, ok := s.configReader.GetBool(context.Background(), key); ok {
		return v
	}
	return fallback
}

func (s *ChatService) AnalyzeFeedback(ctx context.Context, limitDays int) (string, error) {
	if s.chatRepo == nil {
		return "", errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	if s.modelFactory == nil || s.modelFactory.GetModel() == nil {
		return "", errcode.AppError{Code: errcode.ErrAIUnavailable, Message: "LLM 服务未初始化"}
	}

	samples, err := s.chatRepo.FindFeedbackSamples(ctx, limitDays)
	if err != nil {
		return "", fmt.Errorf("查询反馈样本失败: %w", err)
	}
	if len(samples) == 0 {
		return "{\"message\":\"暂无反馈数据可供分析，请先使用问答功能并提交反馈。\"}", nil
	}

	var helpful, unhelpful strings.Builder
	helpfulCount, unhelpfulCount := 0, 0
	for _, s := range samples {
		question := truncateRunes(s.Question, 200)
		answer := truncateRunes(s.Answer, 300)
		if s.Feedback == 1 {
			helpfulCount++
			helpful.WriteString(fmt.Sprintf("- Q: %s\n  A: %s\n", question, answer))
		} else {
			unhelpfulCount++
			unhelpful.WriteString(fmt.Sprintf("- Q: %s\n  A: %s\n", question, answer))
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

	// Eino ChatModel.Generate（非流式）
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


func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

// confidenceLevel 置信度分级（high/medium/low）。Agent 不产生置信度；
// GetChatDetail 展示历史会话（RAG 时期）的置信度。
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
