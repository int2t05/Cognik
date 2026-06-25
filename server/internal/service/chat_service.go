// Package service 实现智能问答业务逻辑。
//
// 会话与对话分离：CreateSession 仅创建容器，StreamChat 在已有会话中流式返回 AI 答案。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"opsmind/internal/adapter"
	"opsmind/internal/dto/request"
	"opsmind/internal/dto/response"
	"opsmind/internal/model"
	"opsmind/internal/rag"
	"opsmind/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	defaultConfidenceThreshold = 0.6
	fallbackLowConfidence      = "暂未找到足够匹配的知识，建议提交申告由运维人员人工处理"
	fallbackAIUnavailable      = "当前 AI 服务暂不可用，请提交申告由人工处理"
)

// 消费者接口——ChatService 仅暴露它实际使用的依赖方法，
// 遵循 Go "accept interfaces, return structs" 惯例，便于测试 mock。
type chatKnowledgeRepo interface {
	FindKBByID(ctx context.Context, id int64) (*model.KnowledgeBase, error)
}

type chatSessionRepo interface {
	Create(ctx context.Context, session *model.ChatSession) error
	CreateBatch(ctx context.Context, messages []model.ChatMessage) error
	CreateMessage(ctx context.Context, m *model.ChatMessage) error
	UpdateMessage(ctx context.Context, m *model.ChatMessage) error
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

// RAGDefaults RAG 管道默认开关（从 env 配置读取）。
type RAGDefaults struct {
	TopK         int
	QueryRewrite bool
	MultiRoute   bool
	Hybrid       bool
	Rerank       bool
}

// ragConfigReader ChatService 需要的运行时配置读取能力。
type ragConfigReader interface {
	GetInt(ctx context.Context, key string) (int, bool)
	GetFloat(ctx context.Context, key string) (float64, bool)
	GetBool(ctx context.Context, key string) (bool, bool)
}

// ChatService 智能问答服务。
type ChatService struct {
	ragDefaults   RAGDefaults
	configReader  ragConfigReader // 运行时读取 DB 配置覆盖 env 默认值
	knowledgeRepo chatKnowledgeRepo
	chatRepo      chatSessionRepo
	llmService    *LLMService
	auditRepo     auditLogWriter // 审计日志写入接口（反馈事件记录）
	hub           *GenerationHub
}


// auditLogWriter ChatService 需要的审计日志最小接口。
type auditLogWriter interface {
	Create(ctx context.Context, log any) error
}

// NewChatService 创建 ChatService 实例。
func NewChatService(knowledgeRepo chatKnowledgeRepo, chatRepo chatSessionRepo, llmService *LLMService, ragDefaults RAGDefaults, configReader ragConfigReader, auditRepo auditLogWriter, hub *GenerationHub) *ChatService {
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

// =============================================================================
// CreateSession — 创建会话容器
// =============================================================================

// CreateSession 创建问答会话（仅创建容器，不含 LLM 调用）。
// 与 StreamChat 分离的原因是：会话生命周期与 AI 调用解耦，避免 LLM 超时阻塞 HTTP 请求。
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

// =============================================================================
// StreamChat — 流式对话（在已有会话中）
// =============================================================================

// StreamChat 发起一次新生成：立即落库用户消息、建 generating 的 assistant 消息，
// 在 context.Background() 跑生成（脱离请求 ctx，客户端断开不影响），
// 返回 Hub 订阅（replay+实时）。完成时由后台 goroutine 落库 assistant 终稿。
// routeCount/rerankCount 为 0 时使用 RAG 管道默认值。
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

	// 加载会话并校验归属
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

	// 加载历史消息（用于 LLM 上下文 + RAG 查询改写消歧）
	var history []adapter.ChatMessage
	msgs, msgErr := s.chatRepo.FindMessagesBySession(ctx, sessionID)
	if msgErr != nil {
		slog.Warn("加载会话历史消息失败，多轮上下文降级为单轮", "session_id", sessionID, "error", msgErr)
	}
	for _, m := range msgs {
		history = append(history, adapter.ChatMessage{Role: m.Role, Content: m.Content})
	}

	// 构建 RAG 查询改写所需的对话历史（格式：[]map[string]string）
	var ragHistory []map[string]string
	for _, m := range msgs {
		if m.Role == "user" || m.Role == "assistant" {
			ragHistory = append(ragHistory, map[string]string{"role": m.Role, "content": m.Content})
		}
	}

	// RAG 管道选项：env 默认值 → DB 配置覆盖 → 请求级参数
	opts := s.buildRAGOptions(routeCount, rerankCount, ragHistory)

	// 立即落库用户消息（解决导航/刷新后用户消息丢失）
	if err := s.chatRepo.CreateMessage(ctx, &model.ChatMessage{
		SessionID: sessionID, Role: "user", Content: question, Status: model.MessageStatusCompleted,
	}); err != nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "保存用户消息失败"}
	}
		// 首次对话时将标题从"新对话"自动更新为用户问题
	if session.Question == "新对话" || session.Question == "" {
	_ = s.chatRepo.UpdateSessionMeta(ctx, sessionID, question, 0)
		}

	// 建 generating 的 assistant 占位消息，拿到 msgID
	assistant := &model.ChatMessage{SessionID: sessionID, Role: "assistant", Content: "", Status: model.MessageStatusGenerating}
	if err := s.chatRepo.CreateMessage(ctx, assistant); err != nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "创建回复占位失败"}
	}

	// 脱离请求 ctx：用 background + 独立超时
	gctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	if err := s.hub.Start(sessionID, assistant.ID, cancel); err != nil {
		cancel()
		// 标记占位失败，避免残留 generating
		assistant.Status = model.MessageStatusFailed
		_ = s.chatRepo.UpdateMessage(context.Background(), assistant)
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrParam, Message: err.Error()}
	}

	go s.runGeneration(gctx, sessionID, assistant.ID, question, session.KBID, opts, history)

	replay, ch, unsub, ok := s.hub.Subscribe(sessionID, 0)
	if !ok {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "订阅生成失败"}
	}
	return replay, ch, unsub, nil
}

// runGeneration 在后台跑 RAG 管道，逐事件 Publish 到 Hub；完成/失败时落库并 Finish。
func (s *ChatService) runGeneration(gctx context.Context, sessionID, msgID int64, question string, kbID int64, opts rag.RAGOptions, history []adapter.ChatMessage) {
	defer s.hub.Finish(sessionID)

	enableThinking := s.readBool("ai.enable_thinking", false)
	llmEvents, err := s.llmService.StreamChat(gctx, question, kbID, opts, history, enableThinking)
	if err != nil {
		s.hub.Publish(sessionID, StreamEvent{Type: "error", Error: err.Error()})
		s.failAssistant(msgID)
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
	_ = s.chatRepo.UpdateSession(context.Background(), &model.ChatSession{
				ID: sessionID, Answer: evt.Metadata.Answer, Sources: srcJSON,
				Confidence: evt.Metadata.Confidence, DurationMs: evt.Metadata.DurationMS,
			})
	_ = s.chatRepo.UpdateMessage(context.Background(), &model.ChatMessage{
				ID: msgID, Content: evt.Metadata.Answer, Sources: srcJSON,
				PipelineMetrics: pipelineJSON, Confidence: evt.Metadata.Confidence,
				Status: model.MessageStatusCompleted,
			})
			evt.Metadata.SessionID = sessionID
			evt.Metadata.Question = question
			evt.Metadata.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
		}
		if evt.Type == "error" {
			s.failAssistant(msgID)
		}
		s.hub.Publish(sessionID, evt)
	}

	// gctx 被取消（用户停止/超时）：落库当前已生成内容为 failed
	if gctx.Err() != nil {
		_ = s.chatRepo.UpdateMessage(context.Background(), &model.ChatMessage{
			ID: msgID, Content: answer, Status: model.MessageStatusFailed,
		})
		s.hub.Publish(sessionID, StreamEvent{Type: "error", Error: "生成已停止"})
	}
}

// failAssistant 将 assistant 消息标记为 failed，用于生成失败时清理占位状态。
func (s *ChatService) failAssistant(msgID int64) {
	_ = s.chatRepo.UpdateMessage(context.Background(), &model.ChatMessage{ID: msgID, Status: model.MessageStatusFailed})
}

// ResumeStream 续传：校验会话归属后从 since 订阅 Hub。无活跃生成则返回 ErrNotFound。
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

// CancelGeneration 校验归属后真正取消后端生成。
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

// CleanupStaleGenerating 启动时把残留 generating 标记 failed。
func (s *ChatService) CleanupStaleGenerating(ctx context.Context) error {
	_, err := s.chatRepo.MarkGeneratingFailed(ctx)
	return err
}

// =============================================================================
// SubmitFeedback
// =============================================================================

// SubmitMessageFeedback 提交单条消息的显式反馈（用户主动点击 👍👎）。
//
// 与 SubmitFeedback（会话级）不同，本方法针对单条 assistant 消息进行反馈。
// 校验：消息必须存在、属于指定会话、且角色为 assistant。
// 反馈成功后写入审计日志。
func (s *ChatService) SubmitMessageFeedback(ctx context.Context, messageID, sessionID, userID int64, feedback int16) error {
	if s.chatRepo == nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	if feedback < 0 || feedback > 2 {
		return errcode.AppError{Code: errcode.ErrParam, Message: "反馈值无效，请输入 0（取消）/1（有帮助）/2（无帮助）"}
	}

	// 校验会话归属
	session, err := s.chatRepo.FindByID(ctx, sessionID)
	if err != nil {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "会话不存在"}
	}
	if session.UserID != userID {
		return errcode.AppError{Code: errcode.ErrForbidden, Message: "无权操作该会话"}
	}

	// 校验消息存在
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

	// 审计日志
	s.writeFeedbackAudit(ctx, userID, sessionID, messageID, feedback, "explicit")
	return nil
}

// MarkLastAssistantUnhelpful 隐式反馈：将指定会话最后一条 assistant 消息标记为"无帮助"。
//
// 触发场景：用户在 AI 回答后提交了申告，意味着 AI 未能解决其问题。
// 这是 FeedbackMarker 接口的实现，供 TicketService 调用。
// 不返回错误（非关键路径），仅写审计日志。
func (s *ChatService) MarkLastAssistantUnhelpful(ctx context.Context, sessionID int64) error {
	if s.chatRepo == nil {
		return nil
	}

	msgs, err := s.chatRepo.FindMessagesBySession(ctx, sessionID)
	if err != nil {
		return err
	}

	// 从后往前找最后一条 assistant 消息
	var lastAssistant *model.ChatMessage
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "assistant" {
			lastAssistant = &msgs[i]
			break
		}
	}
	if lastAssistant == nil {
		return nil // 无 assistant 消息，无需标记
	}
	// 已有点赞的不覆盖（用户手动点赞说明回答有用）
	if lastAssistant.Feedback == 1 {
		return nil
	}

	if err := s.chatRepo.UpdateMessageFeedback(ctx, lastAssistant.ID, 2); err != nil {
		return err
	}

	s.writeFeedbackAudit(ctx, 0, sessionID, lastAssistant.ID, 2, "implicit")
	return nil
}

// writeFeedbackAudit 写入反馈审计日志（异步，不阻塞主流程）。
func (s *ChatService) writeFeedbackAudit(ctx context.Context, userID, sessionID, messageID int64, feedback int16, source string) {
	if s.auditRepo == nil {
		return
	}
	detail, _ := json.Marshal(map[string]any{
		"session_id": sessionID,
		"message_id": messageID,
		"feedback":   feedback,
		"source":     source, // "explicit" | "implicit"
	})
	auditLog := &model.AuditLog{
		OperatorID: userID,
		Action:     "chat.feedback",
		TargetType: "chat_message",
		TargetID:   messageID,
		Detail:     datatypes.JSON(detail),
	}
	// 审计写入失败不阻塞主流程
	if err := s.auditRepo.Create(ctx, auditLog); err != nil {
		slog.Warn("反馈审计日志写入失败", "message_id", messageID, "error", err)
	}
}

// SubmitFeedback 提交问答反馈。
//
// 校验规则在 Service 层集中管理，不依赖 Handler 层参数校验。
func (s *ChatService) SubmitFeedback(ctx context.Context, sessionID int64, userID int64, feedback int16) error {
	if s.chatRepo == nil {
		return errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	if feedback < 0 || feedback > 2 {
		return errcode.AppError{Code: errcode.ErrParam, Message: "反馈值无效，请输入 0（未反馈）/1（已解决）/2（未解决）"}
	}
	// 仅允许从「未反馈」(0) 更新为「已解决」(1) 或「未解决」(2)，禁止用 0 覆盖已有反馈。
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

// =============================================================================
// GetChatDetail
// =============================================================================

// GetChatDetail 查询问答会话详情（含多轮对话消息历史 + 归属校验）。
func (s *ChatService) GetChatDetail(ctx context.Context, sessionID int64, userID int64) (*response.ChatSessionResponse, error) {
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

	var sources []response.SourceItem
	if len(session.Sources) > 0 {
		if err := json.Unmarshal(session.Sources, &sources); err != nil {
			slog.Warn("解析会话 Sources JSON 失败", "session_id", sessionID, "error", err)
		}
	}

	// 加载消息历史
	var messages []response.MessageItem
	if msgs, msgErr := s.chatRepo.FindMessagesBySession(ctx, sessionID); msgErr == nil {
		for _, m := range msgs {
			var msgSources []response.SourceItem
			if len(m.Sources) > 0 {
				if err := json.Unmarshal(m.Sources, &msgSources); err != nil {
				slog.Warn("解析消息 Sources JSON 失败", "message_id", m.ID, "error", err)
				}
			}
			messages = append(messages, response.MessageItem{
				ID:         m.ID,
				Role:       m.Role,
				Content:    m.Content,
				Sources:    msgSources,
				Confidence: m.Confidence,
				Feedback:   m.Feedback,
				Status:     m.Status,
				CreatedAt:  m.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
	}

	return &response.ChatSessionResponse{
		SessionID:       session.ID,
		Question:        session.Question,
		Answer:          session.Answer,
		Sources:         sources,
		Confidence:      session.Confidence,
		CanSubmitTicket: session.Confidence < defaultConfidenceThreshold,
		DurationMS:      session.DurationMs,
		Feedback:        session.Feedback,
		CreatedAt:       session.CreatedAt.Format("2006-01-02 15:04:05"),
		Messages:        messages,
	}, nil
}

// =============================================================================
// ListSessions — 会话列表
// =============================================================================

// ListSessions 分页查询用户的问答会话列表。
//
// 每条会话返回首轮问题标题 + 最后一条回复摘要 + 消息总数。
func (s *ChatService) ListSessions(ctx context.Context, userID int64, page, pageSize int) ([]response.SessionListItem, int64, error) {
	if s.chatRepo == nil {
		return nil, 0, errcode.AppError{Code: errcode.ErrUnknown, Message: "服务未初始化"}
	}
	sessions, total, err := s.chatRepo.ListByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("查询会话列表失败: %w", err)
	}

	// 批量获取消息数量，避免 N+1 查询
	sessionIDs := make([]int64, len(sessions))
	for i, sess := range sessions {
		sessionIDs[i] = sess.ID
	}
	msgCounts, countErr := s.chatRepo.CountMessagesBySessions(ctx, sessionIDs)
	if countErr != nil {
		slog.Warn("批量获取会话消息数失败", "error", countErr)
		msgCounts = map[int64]int64{}
	}

	items := make([]response.SessionListItem, 0, len(sessions))
	for _, sess := range sessions {
		lastAnswer := truncateText(sess.Answer, 100)
		items = append(items, response.SessionListItem{
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

// DeleteSession 删除会话及其全部消息（含归属校验）。
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

// UpdateSessionMeta 更新会话标题和/或所属知识库（含归属校验）。
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

// buildRAGOptions 合并多层配置：env 默认 → DB 运行时配置 → 请求参数。
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


// AnalyzeFeedback 调用 LLM 分析反馈数据，输出知识盲区报告。
//
// 查询最近 limitDays 天内的反馈样本，构建 prompt 让 LLM 分析：
//   - 哪些方面回答得好（strong_areas）
//   - 哪些方面需要补充知识（weak_areas）
//   - 具体的改进建议（suggestions）
//   - 一句话总结（summary）
//
// 返回 JSON 字符串，调用方自行解析。
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

	// 构建分析 prompt：按有帮助/无帮助分组，截断过长内容
	var helpful, unhelpful strings.Builder
	helpfulCount, unhelpfulCount := 0, 0
	for _, s := range samples {
		// 截断长文本避免超出 LLM 上下文
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
  "strong_areas": ["方面1", "方面2"],      // 回答得好的知识领域（根据"有帮助"的问题主题归纳，最多5个）
  "weak_areas": ["方面1", "方面2"],        // 需要补充的知识领域（根据"无帮助"的问题主题归纳，最多5个）
  "suggestions": ["建议1", "建议2"],       // 具体的知识库改进建议（最多5条）
  "summary": "一句话总结（30字以内）"       // 整体评价
}`, helpfulCount, helpful.String(), unhelpfulCount, unhelpful.String())

	client := s.llmService.getLLMClient()
	if client == nil {
		return "", errcode.AppError{Code: errcode.ErrAIUnavailable, Message: "LLM 客户端未初始化"}
	}

	model, maxTokens := s.llmService.getModelConfig()
	resp, err := client.ChatCompletion(ctx, adapter.ChatRequest{
		Messages: []adapter.ChatMessage{
			{Role: "system", Content: "你是运维知识库质量分析师。根据用户反馈数据，识别知识盲区和改进方向。只输出 JSON，不要任何解释。"},
			{Role: "user", Content: prompt},
		},
		Model:          model,
		MaxTokens:      maxTokens,
		Temperature:    0.3,
		EnableThinking: true, // 反馈分析是复杂推理任务，开启思考提升分析质量
	})
	if err != nil {
		return "", fmt.Errorf("LLM 分析调用失败: %w", err)
	}

	return resp.Content, nil
}

// truncateRunes 按 rune 截断文本，超出加 "..."。
func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

// truncateText 截断文本到 maxRunes 个字符，超出加 "..."
func truncateText(text string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}
