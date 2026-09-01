// Package chat 聚合智能问答领域（会话管理、RAG+LLM 编排、LLM 配置）的
// Handler / Service / Repository 三层实现。
//
// service.go 合并原 chat_service / llm_service / llm_config_service 三个 Service。
// ChatService 关注会话生命周期，LLMService 关注 RAG+LLM 编排，LLMConfigService 管理 LLM 配置热替换。
// auditLogWriter / AuditWriter / FeedbackMarker 均通过消费者接口注入——本包只依赖接口而非具体实现，
// Go 结构化类型系统使 system.AuditService 等外部 Service 自动满足这些接口，无需显式 import，避免跨领域循环依赖。
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"opsmind/internal/infra/adapter"
	"opsmind/internal/infra/runtime"
	"opsmind/internal/shared/dto/request"
	"opsmind/internal/shared/dto/response"
	"opsmind/internal/shared/model"
	"opsmind/internal/rag"
	"opsmind/internal/shared/pkg/errcode"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	fallbackLowConfidence = "暂未找到足够匹配的知识，建议提交申告由运维人员人工处理"
	fallbackAIUnavailable = "当前 AI 服务暂不可用，请提交申告由人工处理"
)

// =============================================================================
// 消费者接口——本包只依赖接口而非具体实现，遵循 Go "accept interfaces, return structs" 惯例。
// =============================================================================

// chatKnowledgeRepo 定义 ChatService 所需的知识库查询能力。
type chatKnowledgeRepo interface {
	FindKBByID(ctx context.Context, id int64) (*model.KnowledgeBase, error)
}

// chatSessionRepo 定义 ChatService 所需的会话与消息仓库方法。
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

// ragConfigReader ChatService 需要的运行时配置读取能力。
type ragConfigReader interface {
	GetInt(ctx context.Context, key string) (int, bool)
	GetFloat(ctx context.Context, key string) (float64, bool)
	GetBool(ctx context.Context, key string) (bool, bool)
}

// auditLogWriter ChatService 需要的审计日志最小接口（直接写入完整 AuditLog）。
type auditLogWriter interface {
	Create(ctx context.Context, log any) error
}

// AuditWriter 定义 LLM 配置审计日志写入接口（消费者接口模式）。
// Go 结构化类型系统使任何实现了 Write 方法的类型自动满足此接口，
// 无需显式 import system 包，避免跨领域循环依赖。
type AuditWriter interface {
	Write(ctx context.Context, operatorID int64, action, targetType string, targetID int64, detail string) error
}

// ragPipeline 定义 LLMService 所需的 RAG 管道接口。
type ragPipeline interface {
	Execute(ctx context.Context, query string, kbID int64, opts rag.RAGOptions, onStep rag.StepCallback) (*rag.RAGResult, error)
}

// llmConfigRepo 定义 LLM 配置仓库接口（消费者定义接口）。
type llmConfigRepo interface {
	Create(ctx context.Context, cfg *model.LlmConfig) error
	FindByID(ctx context.Context, id int64) (*model.LlmConfig, error)
	FindDefault(ctx context.Context) (*model.LlmConfig, error)
	List(ctx context.Context) ([]model.LlmConfig, error)
	Update(ctx context.Context, cfg *model.LlmConfig) error
	Delete(ctx context.Context, id int64) error
	ClearDefault(ctx context.Context) error
	CountReferencingKBs(ctx context.Context, configID int64) (int64, error)
}

type txRepoFactory func(tx *gorm.DB) llmConfigRepo

// =============================================================================
// 流式事件类型
// =============================================================================

// StreamEvent 流式响应中的单个事件，JSON 标签对应 SSE 线格式。
type StreamEvent struct {
	Type     string             `json:"type"`               // "step" | "chunks" | "token" | "done" | "error"
	Seq      int                `json:"seq"`                // 生成内单调递增序号，用于断点续传去重
	Content  string             `json:"content,omitempty"`  // token 文本（type=token）或 reasoning 内容
	ID       string             `json:"id,omitempty"`       // 步骤标识（type=step）
	Label    string             `json:"label,omitempty"`   // 步骤显示名（type=step）
	Error    string             `json:"error,omitempty"`   // 错误信息（type=error）
	Chunks   []rag.ChunkDisplay `json:"chunks,omitempty"`   // 检索 chunk 展示分（type=chunks）
	Metadata *StreamDoneMeta    `json:"metadata,omitempty"` // 完成元数据（type=done）
}

// StreamDoneMeta done 事件携带的会话元数据。
type StreamDoneMeta struct {
	SessionID         int64                 `json:"session_id"`
	Question          string                `json:"question"`
	Answer            string                `json:"answer"`
	Sources           []response.SourceItem `json:"sources"`
	Confidence        float64               `json:"confidence"`
	ConfidenceRaw     float64               `json:"confidence_raw"`
	ConfidenceLevel   string                `json:"confidence_level"`
	CanSubmitTicket   bool                  `json:"can_submit_ticket"`
	DurationMS        int                   `json:"duration_ms"`
	Feedback          int16                 `json:"feedback"`
	CreatedAt         string                `json:"created_at"`
	Pipeline          *ChatPipelineMeta     `json:"pipeline,omitempty"`
	UserMessageID     int64                 `json:"user_message_id,omitempty"`
	AssistantMessageID int64                `json:"assistant_message_id,omitempty"`
}

// =============================================================================
// 管道元数据类型
// =============================================================================

// ChatPipelineMeta 管道执行元数据。
type ChatPipelineMeta struct {
	Steps           []ChatPipelineStep `json:"steps"`
	TotalDurationMS int                `json:"total_duration_ms"`
}

// ChatPipelineStep 管道单步骤耗时与状态。
type ChatPipelineStep struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	DurationMS int    `json:"duration_ms"`
	Success    bool   `json:"success"`
}

// =============================================================================
// LLMConfigManager — 配置热替换
// =============================================================================

// LLMConfigManager 管理当前生效的 LLM 配置（热替换）。
//
// onChange 在默认配置变更时被调用，用于触发 LLM/Embedding 客户端重建。
// 如果回调未注册（nil），配置变更仅更新内存缓存，客户端保持不变。
type LLMConfigManager struct {
	current  atomic.Value // *model.LlmConfig
	onChange func()       // 默认配置变更回调
}

func NewLLMConfigManager() *LLMConfigManager {
	return &LLMConfigManager{}
}

// OnChange 注册默认配置变更回调。仅允许注册一次（覆盖式）。
func (m *LLMConfigManager) OnChange(fn func()) {
	m.onChange = fn
}

// GetConfig 返回当前生效的配置（零锁读取），可能为 nil。
func (m *LLMConfigManager) GetConfig() *model.LlmConfig {
	v := m.current.Load()
	if v == nil {
		return nil
	}
	return v.(*model.LlmConfig)
}

// store 原子替换配置并触发变更回调。
func (m *LLMConfigManager) store(cfg *model.LlmConfig) {
	clone := *cfg
	m.current.Store(&clone)
	if m.onChange != nil {
		m.onChange()
	}
}

// =============================================================================
// RAGDefaults
// =============================================================================

// RAGDefaults RAG 管道默认开关（从 env 配置读取）。
type RAGDefaults struct {
	TopK         int
	QueryRewrite bool
	MultiRoute   bool
	Hybrid       bool
	Rerank       bool
}

// =============================================================================
// ChatService — 智能问答服务
// =============================================================================

// ChatService 智能问答服务。
type ChatService struct {
	ragDefaults   RAGDefaults
	configReader  ragConfigReader // 运行时读取 DB 配置覆盖 env 默认值
	knowledgeRepo chatKnowledgeRepo
	chatRepo      chatSessionRepo
	llmService    *LLMService
	auditRepo     auditLogWriter // 审计日志写入接口（反馈事件记录）
	hub           *runtime.GenerationHub[StreamEvent]
}

// NewChatService 创建 ChatService 实例。
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

	// 立即落库用户消息
	userMsg := &model.ChatMessage{SessionID: sessionID, Role: "user", Content: question, Status: model.MessageStatusCompleted}
	if err := s.chatRepo.CreateMessage(ctx, userMsg); err != nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "保存用户消息失败"}
	}
	// 首次对话时将标题从默认值自动更新为用户问题
	if session.Question == "新会话" || session.Question == "" {
		_ = s.chatRepo.UpdateSessionMeta(ctx, sessionID, question, 0)
	}

	// 建 generating 的 assistant 占位消息，拿到 msgID
	assistant := &model.ChatMessage{SessionID: sessionID, Role: "assistant", Content: "", Status: model.MessageStatusGenerating}
	if err := s.chatRepo.CreateMessage(ctx, assistant); err != nil {
		return nil, nil, nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "创建回复占位失败"}
	}

	// 脱离请求 ctx：用 background + 独立超时
	// 思考模式额外增加超时（思考 ~30s + 生成 ~90s）
	genTimeout := 120 * time.Second
	if s.readBool("ai.enable_thinking", false) {
		genTimeout = 300 * time.Second
	}
	gctx, cancel := context.WithTimeout(context.Background(), genTimeout)
	if err := s.hub.Start(sessionID, cancel); err != nil {
		cancel()
		// 标记占位失败，避免残留 generating
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

// runGeneration 在后台跑 RAG 管道，逐事件 Publish 到 Hub；完成/失败时落库并 Finish。
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

	// 超时：落库已生成内容；用户主动停止：删除双方消息（回溯到发送前）
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

	// 进入会话页时清理残留的失败消息对（failed assistant + 对应 user），
	// 避免前端展示无意义的失败状态。失败原因已在生成时通过 toast 通知用户。
	if cleaned, err := s.chatRepo.CleanFailedMessages(ctx, sessionID); err != nil {
		slog.Warn("清理失败消息对失败", "session_id", sessionID, "error", err)
	} else if cleaned > 0 {
		slog.Info("已清理失败消息对", "session_id", sessionID, "pairs", cleaned/2)
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
				ID:               m.ID,
				Role:             m.Role,
				Content:          m.Content,
				Sources:          msgSources,
				Confidence:       m.ConfidenceRaw,
				ConfidenceRaw:    m.ConfidenceRaw,
				ConfidenceLevel:  confidenceLevel(m.ConfidenceRaw),
				Feedback:         m.Feedback,
				Status:           m.Status,
				CreatedAt:        m.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
	}

	return &response.ChatSessionResponse{
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
		lastAnswer := truncateRunes(sess.Answer, 100)
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

func (s *ChatService) readFloat(key string, fallback float64) float64 {
	if s.configReader == nil {
		return fallback
	}
	if v, ok := s.configReader.GetFloat(context.Background(), key); ok {
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

// =============================================================================
// LLMService — RAG + LLM 调用编排
// =============================================================================

// SyncChatResult 非流式问答的返回结果。
type SyncChatResult struct {
	Answer     string
	Sources    []response.SourceItem
	Confidence float64
	Pipeline   *ChatPipelineMeta
}

// LLMService 封装 RAG + LLM 调用编排。StreamChat 用于 SSE 流式，SyncChat 用于非流式。
type LLMService struct {
	mu                 sync.Mutex
	llmClient          adapter.LLMClient
	configMgr          *LLMConfigManager
	defaultModel       string
	pipeline           ragPipeline
	embedder           *rag.Embedder // 用于 S_qa 答案向量化
	maxHistoryMessages int           // 多轮对话历史消息数上限（滑动窗口，默认 10）
	configWarnOnce     sync.Once     // 缺少 DB 配置时仅 Warn 一次
}

// NewLLMService 创建 LLMService 实例。
// maxHistoryMessages 控制注入 prompt 的历史消息数上限（0=不限制，默认 10）。
func NewLLMService(llmClient adapter.LLMClient, configMgr *LLMConfigManager, defaultModel string, pipeline ragPipeline, embedder *rag.Embedder, maxHistoryMessages int) *LLMService {
	if maxHistoryMessages <= 0 {
		maxHistoryMessages = 10 // 默认最近 10 条消息（约 5 轮 Q&A）
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

// SetLLMClient 替换 LLM 客户端（默认配置变更时由回调调用）。
func (s *LLMService) SetLLMClient(client adapter.LLMClient) {
	s.mu.Lock()
	s.llmClient = client
	s.mu.Unlock()
}

// getLLMClient 线程安全地获取当前 LLM 客户端。
func (s *LLMService) getLLMClient() adapter.LLMClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.llmClient
}

// SyncChat 执行 RAG 检索 + LLM 同步生成。
// history 为多轮对话历史，在 RAG 上下文前注入。
func (s *LLMService) SyncChat(ctx context.Context, question string, kbID int64, opts rag.RAGOptions, history []adapter.ChatMessage) (*SyncChatResult, error) {
	start := time.Now()

	// Step 1: RAG 管道检索
	ragResult, pipeMeta, err := s.executeRAG(ctx, question, kbID, opts, nil)
	if err != nil {
		return nil, err
	}

	chunks := ragResult.Chunks

	// Step 2: 无检索结果 → 兜底答案
	if len(chunks) == 0 {
		return &SyncChatResult{
			Answer:     "暂未找到足够匹配的知识，建议提交申告由运维人员人工处理。",
			Confidence: 0,
			Pipeline:   pipeMeta,
		}, nil
	}

	// Step 3: LLM 同步生成（仅当 llmClient 可用）
	var answer string
	if client := s.getLLMClient(); client != nil {
		messages := s.buildMessages(chunks, question, history)
		model, maxTokens := s.getModelConfig()
		llmResp, llmErr := client.ChatCompletion(ctx, adapter.ChatRequest{
			Messages:    messages,
			Model:       model,
			MaxTokens:   maxTokens,
			Temperature: 0.3,
		})
		if llmErr != nil {
			answer = "当前 AI 服务暂不可用，请提交申告由人工处理"
		} else {
			answer = llmResp.Content
		}
	} else {
		// 无 LLM：返回检索内容摘要
		var sb strings.Builder
		for i, c := range chunks {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, c.Content))
		}
		answer = "以下是与您问题相关的知识条目：\n\n" + sb.String()
	}

	// 合并管道耗时与 LLM 生成耗时
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

// 降级常量：RAG 不可用/无结果时的提示前缀和 LLM 不可用时的固定回复。
const (
	ragDisabledNotice  = "\n\n> ⚠️ 当前已关闭知识库检索，以下回答由 AI 直接生成，可能不够准确。\n\n"
	ragNoResultNotice  = "\n\n> ⚠️ 当前暂未找到足够匹配的知识，以下回答由 AI 直接生成，仅供参考。\n\n"
	llmUnavailableText = "抱歉，当前 AI 服务暂不可用。建议您：\n1. 稍后重试\n2. 提交运维申告由人工处理\n3. 联系运维团队获取帮助"
)

// StreamChat 执行 RAG 检索 + LLM 流式生成，返回事件通道供 SSE 代理。
//
// 降级策略（三级）：
//  1. RAG 可用 → 正常检索+生成
//  2. RAG 禁用/无结果 → 发送提示 notice token → 直接 LLM 生成（无知识上下文）
//  3. LLM 不可用 → 返回固定降级语句
//
// history 为多轮对话历史，在 RAG 上下文前注入。
func (s *LLMService) StreamChat(ctx context.Context, question string, kbID int64, opts rag.RAGOptions, history []adapter.ChatMessage, enableThinking bool) (<-chan StreamEvent, error) {
	eventCh := make(chan StreamEvent, 100)

	go func() {
		defer close(eventCh)
		start := time.Now()

		// Step 1: RAG 管道检索（实时发送 step 事件到前端）
		onStep := func(evt rag.StepEvent) {
			sendOrCancel(ctx, eventCh, StreamEvent{Type: "step", ID: evt.ID, Label: evt.Label})
		}
		ragResult, pipeMeta, err := s.executeRAG(ctx, question, kbID, opts, onStep)
		if err != nil {
			// RAG 管道失败 → 降级尝试无知识库 LLM
			slog.Warn("RAG 检索失败，降级为纯 LLM 模式", "error", err)
			ragResult = &rag.RAGResult{} // 确保走 LLM-only 分支
		}
		chunks := ragResult.Chunks

		// 判断是否需要发送降级提示
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

		// 发送 chunks SSE 事件（检索完成后、LLM 生成前）
		if len(ragResult.ChunkDisplays) > 0 {
			sendOrCancel(ctx, eventCh, StreamEvent{Type: "chunks", Chunks: ragResult.ChunkDisplays})
		}

		// Step 2: LLM 流式生成
		if s.getLLMClient() == nil {
			s.sendFallback(ctx, eventCh, start)
			return
		}

		sendOrCancel(ctx, eventCh, StreamEvent{Type: "step", ID: "llm_generate", Label: "LLM 生成"})

		messages := s.buildMessages(chunks, question, history)
		model, maxTokens := s.getModelConfig()
		tokenCh, llmErr := s.getLLMClient().ChatCompletionStream(ctx, adapter.ChatRequest{
			Messages:       messages,
			Model:          model,
			MaxTokens:      maxTokens,
			Temperature:    0.3,
			EnableThinking: enableThinking,
		})
		if llmErr != nil {
			// LLM 不可用 → 降级固定回复
			slog.Error("LLM 流式调用失败，降级固定回复", "error", llmErr)
			s.sendFallback(ctx, eventCh, start)
			return
		}

		// 逐 token 输出 + 缓冲完整答案
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
				// 思考内容透传到前端（保持 SSE 连接活跃，前端显示"思考中..."）
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
		// 空回答降级为固定回复
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

// sendNoticeToken 发送降级提示 notice 作为 token 事件（前端显示为灰色引文）。
func (s *LLMService) sendNoticeToken(ctx context.Context, eventCh chan<- StreamEvent, notice string) {
	sendOrCancel(ctx, eventCh, StreamEvent{Type: "token", Content: notice})
}

// sendFallback 发送 LLM 不可用时的固定降级回复。
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

// executeRAG 执行 RAG 管道检索，返回完整 RAGResult 和管道指标。
//
// 第二个返回值 pipelineMeta 可能为 nil（pipeline 不可用时）。
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
		// 转换 StepMetric → ChatPipelineStep
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

// buildMessages 将 RAG chunk 和历史对话构建为 LLM 请求消息。
// history 按滑动窗口截断（maxHistoryMessages 控制），避免长对话超出上下文窗口。
// 系统提示词优先使用 LLM 配置中的 SystemPrompt，为空时回退到默认提示词。
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

	// 滑动窗口截断历史消息：只保留最近 N 条
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

// getModelConfig 从 LLMConfigManager 读取当前模型和 maxTokens。
//
// 优先级：DB 热配置 > config.yaml 默认值。configMgr 为 nil 或 DB 无默认配置时回退到 defaultModel。
// 缺少 DB 配置时仅 Warn 一次（sync.Once），避免每条消息重复刷屏。
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
// LLMConfigService — LLM 配置管理
// =============================================================================

// LLMConfigService LLM 配置管理服务。
type LLMConfigService struct {
	repo        llmConfigRepo
	newRepo     txRepoFactory
	manager     *LLMConfigManager
	auditWriter AuditWriter
	db          *gorm.DB
}

// NewLLMConfigService 创建 LLMConfigService 实例。
func NewLLMConfigService(repo llmConfigRepo, db *gorm.DB, auditWriter AuditWriter) (*LLMConfigService, error) {
	svc := &LLMConfigService{
		repo:        repo,
		manager:     NewLLMConfigManager(),
		db:          db,
		auditWriter: auditWriter,
	}
	if db != nil {
		svc.newRepo = func(tx *gorm.DB) llmConfigRepo { return NewLlmConfigRepo(tx) }
	}

	if cfg, err := svc.repo.FindDefault(context.Background()); err == nil && cfg != nil {
		svc.manager.store(cfg)
	}

	return svc, nil
}

func (s *LLMConfigService) GetManager() *LLMConfigManager { return s.manager }

// CreateConfig 创建 LLM 配置。is_default=true 时先清空其他默认（事务保证原子性）。
func (s *LLMConfigService) CreateConfig(ctx context.Context, name, llmBaseURL, llmAPIKey, embeddingBaseURL, embeddingAPIKey, llmModel, embeddingModel, systemPrompt string, maxTokens, vectorDimension int, isDefault bool) (*model.LlmConfig, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "名称不能为空"}
	}
	if strings.TrimSpace(llmBaseURL) == "" {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "LLM BaseURL 不能为空"}
	}
	if strings.TrimSpace(llmModel) == "" {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "LLM 模型不能为空"}
	}
	if strings.TrimSpace(embeddingModel) == "" {
		return nil, errcode.AppError{Code: errcode.ErrParam, Message: "Embedding 模型不能为空"}
	}
	if maxTokens <= 0 {
		maxTokens = 8192
	}
	if vectorDimension <= 0 {
		vectorDimension = 1024
	}

	cfg := &model.LlmConfig{
		Name: name, LLMBaseURL: llmBaseURL, LLMAPIKey: llmAPIKey,
		EmbeddingBaseURL: embeddingBaseURL, EmbeddingAPIKey: embeddingAPIKey,
		LLMModel: llmModel, EmbeddingModel: embeddingModel, SystemPrompt: systemPrompt,
		MaxTokens: maxTokens, VectorDimension: vectorDimension, IsDefault: isDefault,
	}

	if s.db != nil && isDefault {
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			txRepo := s.newRepo(tx)
			if err := txRepo.ClearDefault(ctx); err != nil {
				return errcode.AppError{Code: errcode.ErrUnknown, Message: "清空默认配置失败"}
			}
			return txRepo.Create(ctx, cfg)
		})
		if err != nil {
			return nil, err
		}
	} else {
		if isDefault {
			if err := s.repo.ClearDefault(ctx); err != nil {
				return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "清空默认配置失败"}
			}
		}
		if err := s.repo.Create(ctx, cfg); err != nil {
			return nil, errcode.AppError{Code: errcode.ErrUnknown, Message: "创建 LLM 配置失败"}
		}
	}

	fresh, err := s.repo.FindByID(ctx, cfg.ID)
	if err != nil {
		return nil, err
	}
	cfg = fresh
	if isDefault {
		s.manager.store(cfg)
	}
	return cfg, nil
}

// UpdateConfig 更新 LLM 配置。llm_api_key / embedding_api_key 为空时保留数据库原值。
func (s *LLMConfigService) UpdateConfig(ctx context.Context, cfg *model.LlmConfig) error {
	existing, err := s.repo.FindByID(ctx, cfg.ID)
	if err != nil {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "LLM 配置不存在"}
	}
	// API Key 为空时保留原值
	if cfg.LLMAPIKey == "" {
		cfg.LLMAPIKey = existing.LLMAPIKey
	}
	if cfg.EmbeddingAPIKey == "" {
		cfg.EmbeddingAPIKey = existing.EmbeddingAPIKey
	}

	if s.db != nil && cfg.IsDefault {
		err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			txRepo := s.newRepo(tx)
			if err := txRepo.ClearDefault(ctx); err != nil {
				return errcode.AppError{Code: errcode.ErrUnknown, Message: "清空默认配置失败"}
			}
			return txRepo.Update(ctx, cfg)
		})
		if err != nil {
			return err
		}
	} else {
		if cfg.IsDefault {
			if err := s.repo.ClearDefault(ctx); err != nil {
				return errcode.AppError{Code: errcode.ErrUnknown, Message: "清空默认配置失败"}
			}
		}
		if err := s.repo.Update(ctx, cfg); err != nil {
			return errcode.AppError{Code: errcode.ErrUnknown, Message: "更新 LLM 配置失败"}
		}
	}

	if cfg.IsDefault {
		fresh, err := s.repo.FindByID(ctx, cfg.ID)
		if err != nil {
			return err
		}
		cfg = fresh
		s.manager.store(cfg)
	}
	if s.auditWriter != nil {
		s.auditWriter.Write(ctx, 0, "llm_config.update", "llm_config", cfg.ID, "")
	}
	return nil
}

func (s *LLMConfigService) ListConfigs(ctx context.Context) ([]LlmConfigResponse, error) {
	configs, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]LlmConfigResponse, len(configs))
	for i, c := range configs {
		result[i] = LlmConfigResponse{
			ID: c.ID, Name: c.Name,
			LLMBaseURL: c.LLMBaseURL, LLMAPIKey: maskAPIKey(c.LLMAPIKey),
			EmbeddingBaseURL: c.EmbeddingBaseURL, EmbeddingAPIKey: maskAPIKey(c.EmbeddingAPIKey),
			LLMModel: c.LLMModel, EmbeddingModel: c.EmbeddingModel,
			SystemPrompt: c.SystemPrompt, MaxTokens: c.MaxTokens,
			VectorDimension: c.VectorDimension, IsDefault: c.IsDefault,
		}
	}
	return result, nil
}

func (s *LLMConfigService) GetConfig(ctx context.Context, id int64) (*model.LlmConfig, error) {
	cfg, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errcode.AppError{Code: errcode.ErrNotFound, Message: "LLM 配置不存在"}
	}
	return cfg, nil
}

func (s *LLMConfigService) DeleteConfig(ctx context.Context, id int64) error {
	cfg, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return errcode.AppError{Code: errcode.ErrNotFound, Message: "LLM 配置不存在"}
	}
	if cfg.IsDefault {
		return errcode.AppError{Code: errcode.ErrParam, Message: "不能删除默认配置，请先设置其他配置为默认"}
	}
	count, err := s.repo.CountReferencingKBs(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return errcode.AppError{Code: errcode.ErrConflict, Message: "该配置被知识库引用，无法删除"}
	}
	return s.repo.Delete(ctx, id)
}

// =============================================================================
// LlmConfigResponse — 列表响应（API Key 脱敏 + MarshalJSON 二次脱敏）
// =============================================================================

type LlmConfigResponse struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	LLMBaseURL       string `json:"llm_base_url"`
	LLMAPIKey        string `json:"llm_api_key"`
	EmbeddingBaseURL string `json:"embedding_base_url"`
	EmbeddingAPIKey  string `json:"embedding_api_key"`
	LLMModel         string `json:"llm_model"`
	EmbeddingModel   string `json:"embedding_model"`
	SystemPrompt     string `json:"system_prompt"`
	MaxTokens        int    `json:"max_tokens"`
	VectorDimension  int    `json:"vector_dimension"`
	IsDefault        bool   `json:"is_default"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

func (r LlmConfigResponse) MarshalJSON() ([]byte, error) {
	type Alias LlmConfigResponse
	return json.Marshal(&struct {
		*Alias
		LLMAPIKey       string `json:"llm_api_key"`
		EmbeddingAPIKey string `json:"embedding_api_key"`
	}{
		Alias:           (*Alias)(&r),
		LLMAPIKey:       maskAPIKey(r.LLMAPIKey),
		EmbeddingAPIKey: maskAPIKey(r.EmbeddingAPIKey),
	})
}

func NewLlmConfigResponse(cfg *model.LlmConfig) LlmConfigResponse {
	return LlmConfigResponse{
		ID: cfg.ID, Name: cfg.Name,
		LLMBaseURL: cfg.LLMBaseURL, LLMAPIKey: cfg.LLMAPIKey,
		EmbeddingBaseURL: cfg.EmbeddingBaseURL, EmbeddingAPIKey: cfg.EmbeddingAPIKey,
		LLMModel: cfg.LLMModel, EmbeddingModel: cfg.EmbeddingModel,
		MaxTokens: cfg.MaxTokens, VectorDimension: cfg.VectorDimension,
		IsDefault: cfg.IsDefault,
		CreatedAt: cfg.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: cfg.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// TestConnection 测试指定 LLM 配置的连接是否可用。
//
// 为什么放在 Service 而非 Handler：
// LLM 连接测试涉及适配器创建和 API 调用，属于领域逻辑而非 HTTP 管道。
// Handler 只负责解析参数和格式化响应，不应知道 adapter.NewOpenAIClient。
func (s *LLMConfigService) TestConnection(ctx context.Context, id int64) (map[string]any, error) {
	cfg, err := s.GetConfig(ctx, id)
	if err != nil {
		return nil, err
	}

	testClient, err := adapter.NewOpenAIClient(cfg.LLMBaseURL, cfg.LLMAPIKey, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("配置的 BaseURL 无效: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	testReq := adapter.ChatRequest{
		Model:       cfg.LLMModel,
		Messages:    []adapter.ChatMessage{{Role: "user", Content: "ping"}},
		MaxTokens:   1,
		Temperature: 0,
	}

	start := time.Now()
	resp, err := testClient.ChatCompletion(ctx, testReq)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("连接测试失败: %w", err)
	}

	return map[string]any{
		"success":      true,
		"model":        cfg.LLMModel,
		"latency_ms":   latency,
		"test_message": resp.Content,
		"tokens_used":  resp.TokensUsed,
	}, nil
}

// =============================================================================
// 公共辅助函数
// =============================================================================

// sendOrCancel 向 channel 发送事件，同时监听 ctx 取消。
// 返回 false 表示 ctx 已取消，调用方应停止后续发送。
func sendOrCancel(ctx context.Context, ch chan<- StreamEvent, evt StreamEvent) bool {
	select {
	case ch <- evt:
		return true
	case <-ctx.Done():
		return false
	}
}

// extractSources 用综合置信度生成前端展示用的来源列表。
//
// 每个 source 的 Confidence 取自 chunk.ConfRaw（综合置信度 [0,1]），
// 而非原始余弦相似度，确保前端展示与后端评分逻辑一致。
// Sources 以 JSONB 落库到 chat_messages.sources，前端刷新后读到持久化的一致分数。
func extractSources(chunks []rag.RetrievalResult) []response.SourceItem {
	sources := make([]response.SourceItem, len(chunks))
	for i, c := range chunks {
		sources[i] = response.SourceItem{
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

// computeConfidence 计算原始综合分和置信度等级。
//
// 公式：Conf_raw = α × S_retrieval + (1-α) × S_qa
// α 根据答案长度动态调整（短答案 S_qa 噪声大，降低权重）。
// 空答案强制 Conf_raw=0, level="low"。
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

	// 精度钳位
	if confRaw < 0 {
		confRaw = 0
	}
	if confRaw > 1 {
		confRaw = 1
	}

	// 阈值默认值（硬编码兜底，DB 配置可覆盖）
	const defaultLowT, defaultHighT = 0.40, 0.70
	level := "low"
	if confRaw >= defaultHighT {
		level = "high"
	} else if confRaw >= defaultLowT {
		level = "medium"
	}

	return confRaw, level
}

// computeSRetrieval 计算检索聚合分（综合置信度 ConfRaw 的排名加权平均）。
func computeSRetrieval(chunks []rag.RetrievalResult) float64 {
	if len(chunks) == 0 {
		return 0
	}

	var sumWeighted, sumWeights float64
	for i, c := range chunks {
		w := 1.0 / float64(i+1) // rank 从 0 开始，首位权重最高
		score := c.ConfRaw
		if score == 0 {
			// ConfRaw 为 0 时回退到 RawCosineScore（兜底）
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

// computeSQA 计算问答匹配分：question 与 answer embedding 的余弦相似度。
func computeSQA(embedder *rag.Embedder, questionEmb []float32, answer string) float64 {
	vecs, _, err := embedder.Embed(context.Background(), []string{answer}, "")
	if err != nil || len(vecs) == 0 {
		slog.Warn("S_qa embedding 失败，降级为 0", "error", err)
		return 0
	}
	return cosineSimilarity(questionEmb, vecs[0])
}

// cosineSimilarity 计算两个 float32 向量的余弦相似度。
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

// answerLenAlpha 根据答案长度调整 α 权重。
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

// confidenceLevel 根据 Conf_raw 和配置阈值判定置信度等级。
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

// truncateRunes 按 rune 截断文本，超出加 "..."。
func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}
