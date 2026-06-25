// Package handler 实现 HTTP 请求处理。
//
// chat.go 提供智能问答接口（含 SSE 流式）。Handler 层仅做参数解析、调用 Service、SSE 事件代理。
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"opsmind/internal/dto/request"
	"opsmind/internal/service"
	"opsmind/pkg/errcode"
	"opsmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// ChatHandler 智能问答接口。
type ChatHandler struct {
	svc *service.ChatService
}

// NewChatHandler 创建 ChatHandler 实例。
func NewChatHandler(svc *service.ChatService) *ChatHandler {
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
	ctx := c.Request.Context()

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, errcode.ErrUnknown, "当前服务器不支持 SSE 流式输出")
		return
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	eventCh, err := h.svc.StreamChat(ctx, sessionID, req.Question, userID, req.RouteCount, req.RerankCount)
	if err != nil {
		// SSE 头已发送，改用 SSE error 事件返回错误
		writeSSEEvent(c.Writer, service.StreamEvent{Type: "error", Error: err.Error()})
		flusher.Flush()
		return
	}

	rc := http.NewResponseController(c.Writer)
	for evt := range eventCh {
		select {
		case <-ctx.Done():
			return
		default:
		}
		writeSSEEvent(c.Writer, evt)
		flusher.Flush()
		// 每次写入后延长写超时（300s = 5 分钟），适应 llama.cpp CPU 推理的长等待场景
		rc.SetWriteDeadline(time.Now().Add(300 * time.Second))
	}
}
