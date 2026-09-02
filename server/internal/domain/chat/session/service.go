// Package session 封装智能问答会话领域的业务逻辑层。
//
// service.go 实现 ChatService（会话生命周期）与 LLMService（RAG+LLM 编排）。
// auditLogWriter / ragConfigReader / ragPipeline 均通过消费者接口注入——
// 本包只依赖接口而非具体实现，Go 结构化类型系统使外部 Service 自动满足这些接口。
// LLMConfigManager 来自同领域 llm_config 子包，用于配置热替换。
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	llmconfig "opsmind/internal/domain/chat/llm_config"
	"opsmind/internal/infra/adapter"
	"opsmind/internal/infra/runtime"
	"opsmind/internal/rag"
	"opsmind/internal/shared/dto/request"
	respDto "opsmind/internal/shared/dto/response"
	"opsmind/internal/shared/model"
	"opsmind/internal/shared/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	fallbackLowConfidence = "暂未找到足够匹配的知识，建议提交申告由运维人员人工处理"
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

type auditLogWriter interface {
	Create(ctx context.Context, log any) error
}

type ragPipeline interface {
	Execute(ctx context.Context, query string, kbID int64, opts rag.RAGOptions, onStep rag.StepCallback) (*rag.RAGResult, error)
}

// =============================================================================
// 流式事件类型
// =============================================================================

type StreamEvent struct {
	Type     string             `json:"type"`
	Seq      int                `json:"seq"`
	Content  string             `json:"content,omitempty"`
	ID       string             `json:"id,omitempty"`
	Label    string             `json:"label,omitempty"`
	Error    string             `json:"error,omitempty"`
	Chunks   []rag.ChunkDisplay `json:"chunks,omitempty"`
	Metadata *StreamDoneMeta    `json:"metadata,omitempty"`
}

type StreamDoneMeta struct {
	SessionID          int64                `json:"session_id"`
	Question           string               `json:"question"`
	Answer             string               `json:"answer"`
	Sources            []respDto.SourceItem `json:"sources"`
	Confidence         float64              `json:"confidence"`
	ConfidenceRaw      float64              `json:"confidence_raw"`
	ConfidenceLevel    string               `json:"confidence_level"`
	CanSubmitTicket    bool                 `json:"can_submit_ticket"`
	DurationMS         int                  `json:"duration_ms"`
	Feedback           int16                `json:"feedback"`
	CreatedAt          string               `json:"created_at"`
	Pipeline           *ChatPipelineMeta    `json:"pipeline,omitempty"`
	UserMessageID      int64                `json:"user_message_id,omitempty"`
	AssistantMessageID int64                `json:"assistant_message_id,omitempty"`
}

type ChatPipelineMeta struct {
	Steps           []ChatPipelineStep `json:"steps"`
	TotalDurationMS int                `json:"total_duration_ms"`
}

type ChatPipelineStep struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	DurationMS int    `json:"duration_ms"`
	Success    bool   `json:"success"`
}

// RAGDefaults RAG 管道默认开关（从 env 配置读取）。
type RAGDefaults struct {
	TopK         int
	QueryRewrite bool
	MultiRoute   bool
	Hybrid       bool
	Rerank       bool
}

// =============================================================================
// ChatService
// =============================================================================

type ChatService struct {
	ragDefaults   RAGDefaults
	configReader  ragConfigReader
	knowledgeRepo chatKnowledgeRepo
	chatRepo      chatSessionRepo
	llmService    *LLMService
	auditRepo     auditLogWriter
	hub           *runtime.GenerationHub[StreamEvent]
}

func NewChatService(knowledgeRepo chatKnowledgeRepo, chatRepo chatSessionRepo, llmService *LLMService, ragDefaults RAGDefaults, configReader ragConfigReader, auditRepo auditLogWriter, hub *runtime.GenerationHub[StreamEvent]) *ChatService {
	if ragDefaults.TopK <= 0 {
		ragDefaults.TopK = 5
	}
	return &ChatService{
		knowledgeRepo: knowledgeRepo,
		chatRepo:      chatRepo,
		llmService:    llmService,
		ragDefaults:   ragDefaults,
		configReader:  configReader,
		auditRepo:     auditRepo,
		hub:           hub,
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

func (s *ChatService) StreamChat(ctx context.Context, sessionID int64, question string, userID int64, routeCount, rerankCount int) ([]StreamEvent, <-chan StreamEvent, func(), error) {
	if strings.TrimSpace(question) == "" {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrParam, Message: "问题不能为空"}
	}
	if s.llmService == nil {
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

	var history []adapter.ChatMessage
	msgs, msgErr := s.chatRepo.FindMessagesBySession(ctx, sessionID)
	if msgErr != nil {
		slog.Warn("加载会话历史消息失败，多轮上下文降级为单轮", "session_id", sessionID, "error", msgErr)
	}
	for _, m := range msgs {
		history = append(history, adapter.ChatMessage{Role: m.Role, Content: m.Content})
	}

	var ragHistory []map[string]string
	for _, m := range msgs {
		if m.Role == "user" || m.Role == "assistant" {
			ragHistory = append(ragHistory, map[string]string{"role": m.Role, "content": m.Content})
		}
	}

	opts := s.buildRAGOptions(routeCount, rerankCount, ragHistory)

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

	genTimeout := 120 * time.Second
	if s.readBool("ai.enable_thinking", false) {
		genTimeout = 300 * time.Second
	}
	gctx, cancel := context.WithTimeout(context.Background(), genTimeout)
	if err := s.hub.Start(sessionID, cancel); err != nil {
		cancel()
		assistant.Status = model.MessageStatusFailed
		_ = s.chatRepo.UpdateMessage(context.Background(), assistant)
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrParam, Message: err.Error()}
	}

	go s.runGeneration(gctx, sessionID, userMsg.ID, assistant.ID, question, session.KBID, opts, history)

	replay, ch, unsub, ok := s.hub.Subscribe(sessionID, 0)
	if !ok {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "订阅生成失败"}
	}
	return replay, ch, unsub, nil
}

func (s *ChatService) runGeneration(gctx context.Context, sessionID, userMsgID, assistantID int64, question string, kbID int64, opts rag.RAGOptions, history []adapter.ChatMessage) {
	defer s.hub.Finish(sessionID)

	enableThinking := s.readBool("ai.enable_thinking", false)
	llmEvents, err := s.llmService.StreamChat(gctx, question, kbID, opts, history, enableThinking)
	if err != nil {
		s.hub.Publish(sessionID, StreamEvent{Type: "error", Error: err.Error()})
		s.failAssistant(assistantID)
		return
	}

	var answer string
	for evt := range llmEvents {
		if evt.Type == "token" {
			answer += evt.Content
		}
		if evt.Type == "done" && evt.Metadata != nil {
			srcJSON, _ := json.Marshal(evt.Metadata.Sources)
			pipelineJSON, _ := json.Marshal(evt.Metadata.Pipeline)
			confRaw := evt.Metadata.ConfidenceRaw
			_ = s.chatRepo.UpdateSession(context.Background(), &model.ChatSession{
				ID: sessionID, Answer: evt.Metadata.Answer, Sources: srcJSON,
				Confidence: confRaw, DurationMs: evt.Metadata.DurationMS,
			})
			_ = s.chatRepo.UpdateMessage(context.Background(), &model.ChatMessage{
				ID: assistantID, Content: evt.Metadata.Answer, Sources: srcJSON,
				PipelineMetrics: pipelineJSON, ConfidenceRaw: confRaw,
				Status: model.MessageStatusCompleted,
			})
			evt.Metadata.SessionID = sessionID
			evt.Metadata.Question = question
			evt.Metadata.AssistantMessageID = assistantID
			evt.Metadata.UserMessageID = userMsgID
			evt.Metadata.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
		}
		if evt.Type == "error" {
			s.failAssistant(assistantID)
		}
		s.hub.Publish(sessionID, evt)
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
		s.hub.Publish(sessionID, StreamEvent{Type: "error", Error: "生成已停止"})
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
	replay, ch, unsub, ok := s.hub.Subscribe(sessionID, since)
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
	if !s.hub.Cancel(sessionID) {
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
	auditLog := &model.AuditLog{
		OperatorID: userID,
		Action:     "chat.feedback",
		TargetType: "chat_message",
		TargetID:   messageID,
		Detail:     datatypes.JSON(detail),
	}
	if err := s.auditRepo.Create(ctx, auditLog); err != nil {
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

func (s *ChatService) buildRAGOptions(routeCount, rerankCount int, history []map[string]string) rag.RAGOptions {
	ragEnabled := s.readBool("ai.rag_enabled", true)
	return rag.RAGOptions{
		TopK:             s.readInt("ai.top_k", s.ragDefaults.TopK),
		QueryRewrite:     s.readBool("ai.rag_query_rewrite", s.ragDefaults.QueryRewrite),
		MultiRoute:       s.readBool("ai.rag_multi_route", s.ragDefaults.MultiRoute),
		Hybrid:           s.readBool("ai.rag_hybrid", s.ragDefaults.Hybrid),
		Rerank:           s.readBool("ai.rag_rerank", s.ragDefaults.Rerank),
		DisableRetrieval: !ragEnabled,
		RouteCount:       routeCount,
		RerankCount:      rerankCount,
		History:          history,
	}
}

func (s *ChatService) readInt(key string, fallback int) int {
	if s.configReader == nil {
		return fallback
	}
	if v, ok := s.configReader.GetInt(context.Background(), key); ok {
		return v
	}
	return fallback
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

func (s *ChatService) readFloat(key string, fallback float64) float64 {
	if s.configReader == nil {
		return fallback
	}
	if v, ok := s.configReader.GetFloat(context.Background(), key); ok {
		return v
	}
	return fallback
}

func (s *ChatService) AnalyzeFeedback(ctx context.Context, limitDays int) (string, error) {
	if s.chatRepo == nil {
		return "", errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	if s.llmService == nil {
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

	client := s.llmService.getLLMClient()
	if client == nil {
		return "", errcode.AppError{Code: errcode.ErrAIUnavailable, Message: "LLM 客户端未初始化"}
	}

	modelName, maxTokens := s.llmService.getModelConfig()
	llmResp, err := client.ChatCompletion(ctx, adapter.ChatRequest{
		Messages: []adapter.ChatMessage{
			{Role: "system", Content: "你是运维知识库质量分析师。根据用户反馈数据，识别知识盲区和改进方向。只输出 JSON，不要任何解释。"},
			{Role: "user", Content: prompt},
		},
		Model:          modelName,
		MaxTokens:      maxTokens,
		Temperature:    0.3,
		EnableThinking: true,
	})
	if err != nil {
		return "", fmt.Errorf("LLM 分析调用失败: %w", err)
	}

	return llmResp.Content, nil
}

// =============================================================================
// LLMService — RAG + LLM 调用编排
// =============================================================================

type SyncChatResult struct {
	Answer     string
	Sources    []respDto.SourceItem
	Confidence float64
	Pipeline   *ChatPipelineMeta
}

type LLMService struct {
	mu                 sync.Mutex
	llmClient          adapter.LLMClient
	configMgr          *llmconfig.LLMConfigManager
	defaultModel       string
	pipeline           ragPipeline
	embedder           *rag.Embedder
	maxHistoryMessages int
	configWarnOnce     sync.Once
}

func NewLLMService(llmClient adapter.LLMClient, configMgr *llmconfig.LLMConfigManager, defaultModel string, pipeline ragPipeline, embedder *rag.Embedder, maxHistoryMessages int) *LLMService {
	if maxHistoryMessages <= 0 {
		maxHistoryMessages = 10
	}
	return &LLMService{
		llmClient:          llmClient,
		configMgr:          configMgr,
		defaultModel:       defaultModel,
		pipeline:           pipeline,
		embedder:           embedder,
		maxHistoryMessages: maxHistoryMessages,
	}
}

func (s *LLMService) SetLLMClient(client adapter.LLMClient) {
	s.mu.Lock()
	s.llmClient = client
	s.mu.Unlock()
}

func (s *LLMService) getLLMClient() adapter.LLMClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.llmClient
}

func (s *LLMService) SyncChat(ctx context.Context, question string, kbID int64, opts rag.RAGOptions, history []adapter.ChatMessage) (*SyncChatResult, error) {
	start := time.Now()

	ragResult, pipeMeta, err := s.executeRAG(ctx, question, kbID, opts, nil)
	if err != nil {
		return nil, err
	}

	chunks := ragResult.Chunks

	if len(chunks) == 0 {
		return &SyncChatResult{
			Answer:     "暂未找到足够匹配的知识，建议提交申告由运维人员人工处理。",
			Confidence: 0,
			Pipeline:   pipeMeta,
		}, nil
	}

	var answer string
	if client := s.getLLMClient(); client != nil {
		messages := s.buildMessages(chunks, question, history)
		modelName, maxTokens := s.getModelConfig()
		llmResp, llmErr := client.ChatCompletion(ctx, adapter.ChatRequest{
			Messages:    messages,
			Model:       modelName,
			MaxTokens:   maxTokens,
			Temperature: 0.3,
		})
		if llmErr != nil {
			answer = "当前 AI 服务暂不可用，请提交申告由人工处理"
		} else {
			answer = llmResp.Content
		}
	} else {
		var sb strings.Builder
		for i, c := range chunks {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, c.Content))
		}
		answer = "以下是与您问题相关的知识条目：\n\n" + sb.String()
	}

	if pipeMeta != nil {
		pipeMeta.Steps = append(pipeMeta.Steps, ChatPipelineStep{
			ID:         "llm_generate",
			Label:      "LLM 生成",
			DurationMS: int(time.Since(start).Milliseconds()) - pipeMeta.TotalDurationMS,
		})
		pipeMeta.TotalDurationMS = int(time.Since(start).Milliseconds())
	}

	confRaw, _ := s.computeConfidence(chunks, ragResult.QuestionEmbedding, answer)

	return &SyncChatResult{
		Answer:     answer,
		Sources:    extractSources(chunks),
		Confidence: confRaw,
		Pipeline:   pipeMeta,
	}, nil
}

const (
	ragDisabledNotice  = "\n\n> ⚠️ 当前已关闭知识库检索，以下回答由 AI 直接生成，可能不够准确。\n\n"
	ragNoResultNotice  = "\n\n> ⚠️ 当前暂未找到足够匹配的知识，以下回答由 AI 直接生成，仅供参考。\n\n"
	llmUnavailableText = "抱歉，当前 AI 服务暂不可用。建议您：\n1. 稍后重试\n2. 提交运维申告由人工处理\n3. 联系运维团队获取帮助"
)

func (s *LLMService) StreamChat(ctx context.Context, question string, kbID int64, opts rag.RAGOptions, history []adapter.ChatMessage, enableThinking bool) (<-chan StreamEvent, error) {
	eventCh := make(chan StreamEvent, 100)

	go func() {
		defer close(eventCh)
		start := time.Now()

		onStep := func(evt rag.StepEvent) {
			sendOrCancel(ctx, eventCh, StreamEvent{Type: "step", ID: evt.ID, Label: evt.Label})
		}
		ragResult, pipeMeta, err := s.executeRAG(ctx, question, kbID, opts, onStep)
		if err != nil {
			slog.Warn("RAG 检索失败，降级为纯 LLM 模式", "error", err)
			ragResult = &rag.RAGResult{}
		}
		chunks := ragResult.Chunks

		ragDisabled := opts.DisableRetrieval
		ragEmpty := len(chunks) == 0
		if ragDisabled || ragEmpty {
			var notice string
			if ragDisabled {
				notice = ragDisabledNotice
			} else {
				notice = ragNoResultNotice
			}
			s.sendNoticeToken(ctx, eventCh, notice)
			sendOrCancel(ctx, eventCh, StreamEvent{Type: "done", Metadata: &StreamDoneMeta{
				Answer:          "当前未找到足够匹配的知识，无法生成回答。",
				Confidence:      0,
				ConfidenceRaw:   0,
				ConfidenceLevel: "low",
				CanSubmitTicket: true,
				DurationMS:      int(time.Since(start).Milliseconds()),
			}})
			return
		}

		if len(ragResult.ChunkDisplays) > 0 {
			sendOrCancel(ctx, eventCh, StreamEvent{Type: "chunks", Chunks: ragResult.ChunkDisplays})
		}

		if s.getLLMClient() == nil {
			s.sendFallback(ctx, eventCh, start)
			return
		}

		sendOrCancel(ctx, eventCh, StreamEvent{Type: "step", ID: "llm_generate", Label: "LLM 生成"})

		messages := s.buildMessages(chunks, question, history)
		modelName, maxTokens := s.getModelConfig()
		tokenCh, llmErr := s.getLLMClient().ChatCompletionStream(ctx, adapter.ChatRequest{
			Messages:       messages,
			Model:          modelName,
			MaxTokens:      maxTokens,
			Temperature:    0.3,
			EnableThinking: enableThinking,
		})
		if llmErr != nil {
			slog.Error("LLM 流式调用失败，降级固定回复", "error", llmErr)
			s.sendFallback(ctx, eventCh, start)
			return
		}

		var answerBuf strings.Builder
	streamLoop:
		for chunk := range tokenCh {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if chunk.Error != nil {
				if answerBuf.Len() > 0 {
					partialAnswer := answerBuf.String()
					confRaw, confLevel := s.computeConfidence(chunks, ragResult.QuestionEmbedding, partialAnswer)
					sendOrCancel(ctx, eventCh, StreamEvent{Type: "done", Metadata: &StreamDoneMeta{
						Answer:          answerBuf.String(),
						Sources:         extractSources(chunks),
						Confidence:      confRaw,
						ConfidenceRaw:   confRaw,
						ConfidenceLevel: confLevel,
						CanSubmitTicket: confLevel != "high",
						DurationMS:      int(time.Since(start).Milliseconds()),
					}})
				} else {
					s.sendFallback(ctx, eventCh, start)
				}
				return
			}
			if chunk.Reasoning != "" {
				sendOrCancel(ctx, eventCh, StreamEvent{Type: "reasoning", Content: chunk.Reasoning})
			}
			if chunk.Content != "" {
				answerBuf.WriteString(chunk.Content)
				if ok := sendOrCancel(ctx, eventCh, StreamEvent{Type: "token", Content: chunk.Content}); !ok {
					return
				}
			}
			if chunk.FinishReason != "" {
				break streamLoop
			}
		}

		fullAnswer := answerBuf.String()
		if strings.TrimSpace(fullAnswer) == "" {
			s.sendFallback(ctx, eventCh, start)
			return
		}

		sources := extractSources(chunks)
		confRaw, confLevel := s.computeConfidence(chunks, ragResult.QuestionEmbedding, fullAnswer)
		durationMS := int(time.Since(start).Milliseconds())
		if pipeMeta != nil {
			pipeMeta.TotalDurationMS = durationMS
		}

		sendOrCancel(ctx, eventCh, StreamEvent{Type: "done", Metadata: &StreamDoneMeta{
			Answer:          fullAnswer,
			Sources:         sources,
			Confidence:      confRaw,
			ConfidenceRaw:   confRaw,
			ConfidenceLevel: confLevel,
			CanSubmitTicket: confLevel != "high",
			DurationMS:      durationMS,
			Pipeline:        pipeMeta,
		}})
	}()

	return eventCh, nil
}

func (s *LLMService) sendNoticeToken(ctx context.Context, eventCh chan<- StreamEvent, notice string) {
	sendOrCancel(ctx, eventCh, StreamEvent{Type: "token", Content: notice})
}

func (s *LLMService) sendFallback(ctx context.Context, eventCh chan<- StreamEvent, start time.Time) {
	sendOrCancel(ctx, eventCh, StreamEvent{Type: "token", Content: llmUnavailableText})
	sendOrCancel(ctx, eventCh, StreamEvent{Type: "done", Metadata: &StreamDoneMeta{
		Answer:          llmUnavailableText,
		Confidence:      0,
		ConfidenceRaw:   0,
		ConfidenceLevel: "low",
		CanSubmitTicket: true,
		DurationMS:      int(time.Since(start).Milliseconds()),
	}})
}

func (s *LLMService) executeRAG(ctx context.Context, question string, kbID int64, opts rag.RAGOptions, onStep rag.StepCallback) (*rag.RAGResult, *ChatPipelineMeta, error) {
	if s.pipeline == nil {
		return nil, nil, nil
	}

	var steps []ChatPipelineStep
	start := time.Now()

	result, err := s.pipeline.Execute(ctx, question, kbID, opts, onStep)
	if err != nil {
		return nil, nil, fmt.Errorf("知识检索失败: %w", err)
	}

	if result != nil {
		for _, m := range result.Metrics.Steps {
			steps = append(steps, ChatPipelineStep{
				ID:         m.StepID,
				Label:      m.Label,
				DurationMS: int(m.DurationMS),
				Success:    m.Success,
			})
		}
		return result, &ChatPipelineMeta{
			Steps:           steps,
			TotalDurationMS: int(time.Since(start).Milliseconds()),
		}, nil
	}

	return nil, nil, nil
}

func (s *LLMService) buildMessages(chunks []rag.RetrievalResult, question string, history []adapter.ChatMessage) []adapter.ChatMessage {
	systemPrompt := "你是企业运维知识助手。严格仅根据下方「知识库内容」回答，禁止使用外部知识。\n\n规则：\n1. 每条事实后必须标注来源编号，如 [1]、[2]。无编号的回答视为无效\n2. 知识库有答案 → 原文复述，不编造细节\n3. 知识库无相关信息 → 只回复「当前知识库未收录此问题，建议提交申告由运维人员处理」\n4. 回答简洁，列表/步骤优先，不闲聊"
	if s.configMgr != nil {
		if cfg := s.configMgr.GetConfig(); cfg != nil && cfg.SystemPrompt != "" {
			systemPrompt = cfg.SystemPrompt
		}
	}
	var ctxBuilder strings.Builder
	for i, chunk := range chunks {
		ctxBuilder.WriteString(fmt.Sprintf("[%d] %s\n", i+1, chunk.Content))
	}

	msgs := []adapter.ChatMessage{
		{Role: "system", Content: systemPrompt},
	}

	if s.maxHistoryMessages > 0 && len(history) > s.maxHistoryMessages {
		history = history[len(history)-s.maxHistoryMessages:]
	}
	for _, h := range history {
		msgs = append(msgs, h)
	}

	msgs = append(msgs, adapter.ChatMessage{
		Role: "user", Content: fmt.Sprintf("知识库内容：\n%s\n\n用户问题：%s", ctxBuilder.String(), question),
	})

	return msgs
}

func (s *LLMService) getModelConfig() (model string, maxTokens int) {
	model = s.defaultModel
	maxTokens = 2048
	if s.configMgr != nil {
		if cfg := s.configMgr.GetConfig(); cfg != nil {
			if cfg.LLMModel != "" {
				model = cfg.LLMModel
			}
			if cfg.MaxTokens > 0 {
				maxTokens = cfg.MaxTokens
			}
			return
		}
	}
	s.configWarnOnce.Do(func() {
		slog.Info("LLM 配置使用 config.yaml 默认值（DB 中未设置默认 LLM 配置）", "model", model)
	})
	return
}

// =============================================================================
// 公共辅助函数
// =============================================================================

func sendOrCancel(ctx context.Context, ch chan<- StreamEvent, evt StreamEvent) bool {
	select {
	case ch <- evt:
		return true
	case <-ctx.Done():
		return false
	}
}

func extractSources(chunks []rag.RetrievalResult) []respDto.SourceItem {
	sources := make([]respDto.SourceItem, len(chunks))
	for i, c := range chunks {
		sources[i] = respDto.SourceItem{
			DocName:      fmt.Sprintf("来源 %d", i+1),
			ChunkContent: c.Content,
			Confidence:   c.ConfRaw,
		}
	}
	return sources
}

// =============================================================================
// 置信度计算
// =============================================================================

func (s *LLMService) computeConfidence(chunks []rag.RetrievalResult, questionEmb []float32, answer string) (float64, string) {
	if strings.TrimSpace(answer) == "" {
		return 0, "low"
	}

	sRetrieval := computeSRetrieval(chunks)
	sQA := 0.0
	if len(questionEmb) > 0 && s.embedder != nil {
		sQA = computeSQA(s.embedder, questionEmb, answer)
	}

	alpha := answerLenAlpha(len([]rune(answer)))
	confRaw := alpha*sRetrieval + (1-alpha)*sQA

	if confRaw < 0 {
		confRaw = 0
	}
	if confRaw > 1 {
		confRaw = 1
	}

	const defaultLowT, defaultHighT = 0.40, 0.70
	level := "low"
	if confRaw >= defaultHighT {
		level = "high"
	} else if confRaw >= defaultLowT {
		level = "medium"
	}

	return confRaw, level
}

func computeSRetrieval(chunks []rag.RetrievalResult) float64 {
	if len(chunks) == 0 {
		return 0
	}

	var sumWeighted, sumWeights float64
	for i, c := range chunks {
		w := 1.0 / float64(i+1)
		score := c.ConfRaw
		if score == 0 {
			score = c.RawCosineScore
		}
		sumWeighted += w * score
		sumWeights += w
	}
	if sumWeights == 0 {
		return 0
	}
	return sumWeighted / sumWeights
}

func computeSQA(embedder *rag.Embedder, questionEmb []float32, answer string) float64 {
	vecs, _, err := embedder.Embed(context.Background(), []string{answer}, "")
	if err != nil || len(vecs) == 0 {
		slog.Warn("S_qa embedding 失败，降级为 0", "error", err)
		return 0
	}
	return cosineSimilarity(questionEmb, vecs[0])
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func answerLenAlpha(length int) float64 {
	switch {
	case length >= 20:
		return 0.7
	case length >= 5:
		return 0.85
	default:
		return 1.0
	}
}

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

func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}
