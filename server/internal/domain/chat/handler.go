// Package chat 聚合智能问答领域（会话管理、RAG+LLM 编排、LLM 配置）的
// Handler / Service / Repository 三层实现。
//
// handler.go 合并原 chat / llm_config 两个 Handler。
// Handler 层职责：参数解析、调用 Service、格式化响应，不包含业务规则。
// parsePagination / getCurrentUserID / handleServiceError 为本领域 Handler 自用的本地副本，
// 与 handler/common.go 中的同名函数行为一致——领域包独立编译，不依赖 handler 包。
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"opsmind/internal/shared/dto/request"
	"opsmind/internal/shared/model"
	"opsmind/internal/shared/pkg/errcode"
	"opsmind/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

// =============================================================================
// Handler 共享工具
// =============================================================================

// parsePagination 从查询参数中解析分页参数（page, pageSize）。
func parsePagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	return page, pageSize
}

// parseID 从路径参数中解析 int64 ID，解析失败时自动返回错误响应。
func parseID(c *gin.Context, key string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的 "+key)
		return 0, false
	}
	return id, true
}

// getCurrentUserID 从 Gin context 中获取当前用户 ID。
func getCurrentUserID(c *gin.Context) (int64, bool) {
	if val, exists := c.Get("userID"); exists {
		if id, ok := val.(int64); ok {
			return id, true
		}
	}
	return 0, false
}

// handleServiceError 统一处理 Service 层错误。
//
// AppError 类型提取业务码，其他错误视为 500。
// 直接使用 errcode.AppError 而非 service.AppError 别名，避免 handler → service 跨层依赖。
func handleServiceError(c *gin.Context, err error) {
	var appErr errcode.AppError
	if errors.As(err, &appErr) {
		response.Error(c, appErr.Code, appErr.Message)
		return
	}
	// 非 AppError 说明是未预期的内部错误，记录真实原因方便排查
	slog.Error("未预期的服务错误", "path", c.Request.URL.Path, "error", err)
	response.Error(c, errcode.ErrUnknown, "服务器内部错误")
}

// =============================================================================
// ChatHandler — 智能问答接口
// =============================================================================

// ChatHandler 智能问答接口。
type ChatHandler struct {
	svc *ChatService
}

// NewChatHandler 创建 ChatHandler 实例。
func NewChatHandler(svc *ChatService) *ChatHandler {
	return &ChatHandler{svc: svc}
}

// =============================================================================
// 会话 CRUD
// =============================================================================

// CreateChatSession 创建问答会话（仅创建容器，不含 LLM 调用）。
//
// POST /api/v1/portal/chat-sessions
func (h *ChatHandler) CreateChatSession(c *gin.Context) {
	var req request.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)
	session, err := h.svc.CreateSession(c.Request.Context(), req, userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, gin.H{
		"session_id": session.ID,
		"kb_id":      session.KBID,
		"question":   session.Question,
		"created_at": session.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

// ListSessions 查询当前用户的问答会话列表。
//
// GET /api/v1/portal/chat-sessions
func (h *ChatHandler) ListSessions(c *gin.Context) {
	userID, _ := getCurrentUserID(c)
	page, pageSize := parsePagination(c)

	items, total, err := h.svc.ListSessions(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.SuccessWithPage(c, items, total, page, pageSize)
}

// DeleteSession 删除会话及其全部消息。
//
// DELETE /api/v1/portal/chat-sessions/:id
func (h *ChatHandler) DeleteSession(c *gin.Context) {
	userID, _ := getCurrentUserID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	if err := h.svc.DeleteSession(c.Request.Context(), id, userID); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
}

// UpdateSessionMeta 更新会话标题和/或知识库。
//
// PATCH /api/v1/portal/chat-sessions/:id
func (h *ChatHandler) UpdateSessionMeta(c *gin.Context) {
	userID, _ := getCurrentUserID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req struct {
		Question string `json:"title"`
		KBID     int64  `json:"kb_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败")
		return
	}

	if err := h.svc.UpdateSessionMeta(c.Request.Context(), id, userID, req.Question, req.KBID); err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, nil)
}

// SubmitFeedback 提交问答反馈。
//
// POST /api/v1/portal/chat-sessions/:id/feedback
// 校验规则下沉到 Service 层集中管理。
func (h *ChatHandler) SubmitFeedback(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的会话 ID")
		return
	}

	var req request.SubmitFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)
	if err := h.svc.SubmitFeedback(c.Request.Context(), id, userID, req.Feedback); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
}

// SubmitMessageFeedback 提交单条消息的反馈（点赞/倒赞）。
//
// POST /api/v1/portal/chat-sessions/:id/messages/:msgId/feedback
//
// 与会话级反馈不同，本端点针对单条 AI 回答进行反馈，
// 支持 0（取消）/1（有帮助）/2（无帮助）。
func (h *ChatHandler) SubmitMessageFeedback(c *gin.Context) {
	idStr := c.Param("id")
	sessionID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的会话 ID")
		return
	}

	msgIDStr := c.Param("msgId")
	messageID, err := strconv.ParseInt(msgIDStr, 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的消息 ID")
		return
	}

	var req request.SubmitFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)
	if err := h.svc.SubmitMessageFeedback(c.Request.Context(), messageID, sessionID, userID, req.Feedback); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
}

// AnalyzeFeedback 触发 LLM 分析反馈数据，输出知识盲区报告。
//
// POST /api/v1/admin/feedback/analyze
// Body: {"days": 30} — 分析最近 N 天的反馈样本（默认 30，上限 365）。
func (h *ChatHandler) AnalyzeFeedback(c *gin.Context) {
	var req struct {
		Days int `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Days = 30
	}
	if req.Days <= 0 {
		req.Days = 30
	}
	if req.Days > 365 {
		response.Error(c, errcode.ErrParam, "天数不能超过365")
		return
	}

	result, err := h.svc.AnalyzeFeedback(c.Request.Context(), req.Days)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, gin.H{"analysis": result})
}

// GetChatDetail 查询问答会话详情（含归属校验）。
//
// GET /api/v1/portal/chat-sessions/:id
func (h *ChatHandler) GetChatDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的会话 ID")
		return
	}

	userID, _ := getCurrentUserID(c)
	resp, err := h.svc.GetChatDetail(c.Request.Context(), id, userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// =============================================================================
// SSE 流式对话
// =============================================================================

// writeSSEEvent 将事件序列化为 JSON 并以 SSE data 帧格式写入。
// 使用 json.Marshal 而非字符串拼接，自动处理控制字符转义。
func writeSSEEvent(w gin.ResponseWriter, evt any) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", string(data))
	return err
}

// StreamChatMessage 在已有会话中发送消息并以 SSE 流式返回 AI 答案。
//
// POST /api/v1/portal/chat-sessions/:id/stream
//
// 与 CreateChatSession 配合：先创建会话，再通过此端点流式对话。
// 生成已脱离本请求生命周期——客户端断开后生成继续运行，通过续传接口可重连。
func (h *ChatHandler) StreamChatMessage(c *gin.Context) {
	idStr := c.Param("id")
	sessionID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的会话 ID")
		return
	}

	var req request.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)

	replay, ch, unsub, err := h.svc.StreamChat(c.Request.Context(), sessionID, req.Question, userID, req.RouteCount, req.RerankCount)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	writeStream(c, replay, ch, unsub)
}

// ResumeStream 续传进行中的生成（GET ?since=N）。
//
// GET /api/v1/portal/chat-sessions/:id/stream?since=N
//
// 用于页面刷新、网络中断后重新接上进行中的 SSE 流。
// since 指定已收到的最大 Seq，Service 层负责过滤已发送事件。
func (h *ChatHandler) ResumeStream(c *gin.Context) {
	sessionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的会话 ID")
		return
	}
	userID, _ := getCurrentUserID(c)
	since, _ := strconv.Atoi(c.DefaultQuery("since", "0"))

	replay, ch, unsub, err := h.svc.ResumeStream(c.Request.Context(), sessionID, userID, since)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	writeStream(c, replay, ch, unsub)
}

// CancelGeneration 停止后端生成（POST）。
//
// POST /api/v1/portal/chat-sessions/:id/cancel
//
// 由前端取消按钮触发，真正中断 LLM 生成 goroutine。
// 与客户端断开不同：断开不停止生成，取消会终止生成。
func (h *ChatHandler) CancelGeneration(c *gin.Context) {
	sessionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的会话 ID")
		return
	}
	userID, _ := getCurrentUserID(c)

	if err := h.svc.CancelGeneration(c.Request.Context(), sessionID, userID); err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, nil)
}

// writeStream 把订阅到的历史回放事件 + 实时事件写到 SSE 客户端。
//
// 客户端断开时通过 unsub 退订（不影响后端生成），由 c.Request.Context().Done() 感知断开。
// 使用 http.NewResponseController 每次 flush 后延长写超时，避免长 SSE 流被 WriteTimeout 截断。
func writeStream(c *gin.Context, replay []StreamEvent, ch <-chan StreamEvent, unsub func()) {
	defer unsub()

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, errcode.ErrUnknown, "当前服务器不支持 SSE 流式输出")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	for _, evt := range replay {
		_ = writeSSEEvent(c.Writer, evt)
	}
	flusher.Flush()

	rc := http.NewResponseController(c.Writer)
	// 心跳：RAG 管道执行时可能数秒无事件，定期发 SSE 注释防浏览器/代理断开
	heartbeat := time.NewTicker(5 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return // 客户端断开：退订，生成继续
		case <-heartbeat.C:
			// SSE 注释行：浏览器忽略但保持 TCP 连接活跃
			fmt.Fprintf(c.Writer, ": heartbeat\n\n")
			flusher.Flush()
			rc.SetWriteDeadline(time.Now().Add(30 * time.Second))
		case evt, ok := <-ch:
			if !ok {
				return // 通道关闭：生成结束
			}
			_ = writeSSEEvent(c.Writer, evt)
			flusher.Flush()
			// 每次写入后延长写超时，保证长 SSE 流不被 WriteTimeout 截断
			rc.SetWriteDeadline(time.Now().Add(30 * time.Second))
		}
	}
}

// =============================================================================
// LLMConfigHandler — LLM 配置管理接口
// =============================================================================

// LLMConfigHandler LLM 配置管理接口。
type LLMConfigHandler struct {
	svc llmConfigService
}

// llmConfigService 定义 Handler 需要的 Service 方法（消费者定义接口）。
type llmConfigService interface {
	CreateConfig(ctx context.Context, name, llmBaseURL, llmAPIKey, embeddingBaseURL, embeddingAPIKey, llmModel, embeddingModel, systemPrompt string, maxTokens, vectorDimension int, isDefault bool) (*model.LlmConfig, error)
	ListConfigs(ctx context.Context) ([]LlmConfigResponse, error)
	GetConfig(ctx context.Context, id int64) (*model.LlmConfig, error)
	UpdateConfig(ctx context.Context, cfg *model.LlmConfig) error
	DeleteConfig(ctx context.Context, id int64) error
	TestConnection(ctx context.Context, id int64) (map[string]any, error)
	GetManager() *LLMConfigManager
}

// NewLLMConfigHandler 创建 LLMConfigHandler 实例。
func NewLLMConfigHandler(svc llmConfigService) *LLMConfigHandler {
	return &LLMConfigHandler{svc: svc}
}

// =============================================================================
// LLM 配置 CRUD 端点
// =============================================================================

// ListConfigs 列出全部 LLM 配置。
//
// GET /api/v1/admin/llm-configs
func (h *LLMConfigHandler) ListConfigs(c *gin.Context) {
	configs, err := h.svc.ListConfigs(c.Request.Context())
	if err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, configs)
}

// CreateConfig 创建 LLM 配置。
//
// POST /api/v1/admin/llm-configs
func (h *LLMConfigHandler) CreateConfig(c *gin.Context) {
	var req request.CreateLLMConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	cfg, err := h.svc.CreateConfig(c.Request.Context(), req.Name, req.LLMBaseURL, req.LLMAPIKey, req.EmbeddingBaseURL, req.EmbeddingAPIKey,
		req.LLMModel, req.EmbeddingModel, req.SystemPrompt, req.MaxTokens, req.VectorDimension, req.IsDefault)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, cfg)
}

// GetConfig 获取单个 LLM 配置详情。
//
// GET /api/v1/admin/llm-configs/:id
func (h *LLMConfigHandler) GetConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的配置 ID")
		return
	}

	cfg, err := h.svc.GetConfig(c.Request.Context(), id)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, cfg)
}

// UpdateConfig 更新 LLM 配置。
//
// PUT /api/v1/admin/llm-configs/:id
func (h *LLMConfigHandler) UpdateConfig(c *gin.Context) {
	// api_key 为空时 Service 层自动保留原值，无需 Handler 处理
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的配置 ID")
		return
	}

	var req request.UpdateLLMConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	cfg := &model.LlmConfig{
		ID:               id,
		Name:             req.Name,
		LLMBaseURL:       req.LLMBaseURL,
		LLMAPIKey:        req.LLMAPIKey,
		EmbeddingBaseURL: req.EmbeddingBaseURL,
		EmbeddingAPIKey:  req.EmbeddingAPIKey,
		LLMModel:         req.LLMModel,
		EmbeddingModel:   req.EmbeddingModel,
		SystemPrompt:     req.SystemPrompt,
		MaxTokens:        req.MaxTokens,
		VectorDimension:  req.VectorDimension,
		IsDefault:        req.IsDefault,
	}

	if err := h.svc.UpdateConfig(c.Request.Context(), cfg); err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, nil)
}

// DeleteConfig 删除 LLM 配置。
//
// DELETE /api/v1/admin/llm-configs/:id
func (h *LLMConfigHandler) DeleteConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的配置 ID")
		return
	}

	if err := h.svc.DeleteConfig(c.Request.Context(), id); err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, nil)
}

// TestConnection 测试指定 LLM 配置的连接。
//
// POST /api/v1/admin/llm-configs/:id/test
// 委托给 LlmConfigService.TestConnection（Handler 不直接创建适配器或调用 LLM）。
func (h *LLMConfigHandler) TestConnection(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的配置 ID")
		return
	}

	result, err := h.svc.TestConnection(c.Request.Context(), id)
	if err != nil {
		response.Error(c, errcode.ErrAIUnavailable, err.Error())
		return
	}

	response.Success(c, result)
}
