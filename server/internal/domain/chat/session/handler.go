// Package session 问答会话 HTTP 请求处理。
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"opsmind/internal/shared/dto/request"
	"opsmind/internal/shared/pkg/errcode"
	resp "opsmind/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

// parsePagination 解析分页参数。
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

// parseID 从路径参数解析 int64 ID，失败时返回错误响应。
func parseID(c *gin.Context, key string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil {
		resp.Error(c, errcode.ErrParam, "无效的 "+key)
		return 0, false
	}
	return id, true
}

// getCurrentUserID 从 Gin context 获取当前用户 ID。
func getCurrentUserID(c *gin.Context) (int64, bool) {
	if val, exists := c.Get("userID"); exists {
		if id, ok := val.(int64); ok {
			return id, true
		}
	}
	return 0, false
}

// handleServiceError 统一处理 Service 错误。
func handleServiceError(c *gin.Context, err error) {
	var appErr errcode.AppError
	if errors.As(err, &appErr) {
		resp.Error(c, appErr.Code, appErr.Message)
		return
	}
	slog.Error("未预期的服务错误", "path", c.Request.URL.Path, "error", err)
	resp.Error(c, errcode.ErrUnknown, "服务器内部错误")
}

// ChatHandler 智能问答接口。
type ChatHandler struct {
	svc *ChatService
}

// NewChatHandler 创建 ChatHandler 实例。
func NewChatHandler(svc *ChatService) *ChatHandler {
	return &ChatHandler{svc: svc}
}

// CreateChatSession 创建问答会话（仅创建容器，不含 LLM 调用）。
//
// POST /api/v1/portal/chat-sessions
func (h *ChatHandler) CreateChatSession(c *gin.Context) {
	var req request.CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)
	session, err := h.svc.CreateSession(c.Request.Context(), req, userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, gin.H{
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

	resp.SuccessWithPage(c, items, total, page, pageSize)
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

	resp.Success(c, nil)
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
		resp.Error(c, errcode.ErrParam, "参数校验失败")
		return
	}

	if err := h.svc.UpdateSessionMeta(c.Request.Context(), id, userID, req.Question, req.KBID); err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, nil)
}

// SubmitFeedback 提交问答反馈。
//
// POST /api/v1/portal/chat-sessions/:id/feedback
func (h *ChatHandler) SubmitFeedback(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Error(c, errcode.ErrParam, "无效的会话 ID")
		return
	}

	var req request.SubmitFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)
	if err := h.svc.SubmitFeedback(c.Request.Context(), id, userID, req.Feedback); err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, nil)
}

// SubmitMessageFeedback 提交单条消息的反馈（点赞/倒赞）。
//
// POST /api/v1/portal/chat-sessions/:id/messages/:msgId/feedback
func (h *ChatHandler) SubmitMessageFeedback(c *gin.Context) {
	idStr := c.Param("id")
	sessionID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Error(c, errcode.ErrParam, "无效的会话 ID")
		return
	}

	msgIDStr := c.Param("msgId")
	messageID, err := strconv.ParseInt(msgIDStr, 10, 64)
	if err != nil {
		resp.Error(c, errcode.ErrParam, "无效的消息 ID")
		return
	}

	var req request.SubmitFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)
	if err := h.svc.SubmitMessageFeedback(c.Request.Context(), messageID, sessionID, userID, req.Feedback); err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, nil)
}

// AnalyzeFeedback 触发 LLM 分析反馈数据，输出知识盲区报告。
//
// POST /api/v1/admin/feedback/analyze
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
		resp.Error(c, errcode.ErrParam, "天数不能超过365")
		return
	}

	result, err := h.svc.AnalyzeFeedback(c.Request.Context(), req.Days)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, gin.H{"analysis": result})
}

// GetChatDetail 查询问答会话详情（含归属校验）。
//
// GET /api/v1/portal/chat-sessions/:id
func (h *ChatHandler) GetChatDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Error(c, errcode.ErrParam, "无效的会话 ID")
		return
	}

	userID, _ := getCurrentUserID(c)
	detail, err := h.svc.GetChatDetail(c.Request.Context(), id, userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, detail)
}

// =============================================================================
// SSE 流式对话
// =============================================================================

// writeSSEEvent 将事件序列化为 SSE 帧：id: {seq}\ndata: {json}\n\n
// id 字段供 SSE 标准 Last-Event-ID 重连。
// 注：前端 ChatStreamProvider 用 fetch+ReadableStream 只解析 data: 行，id: 被过滤忽略，不破坏前端。
func writeSSEEvent(w gin.ResponseWriter, evt StreamEvent) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %d\ndata: %s\n\n", evt.Seq, string(data))
	return err
}

// StreamChatMessage 在已有会话中发送消息并以 SSE 流式返回 AI 答案。
//
// POST /api/v1/portal/chat-sessions/:id/stream
func (h *ChatHandler) StreamChatMessage(c *gin.Context) {
	idStr := c.Param("id")
	sessionID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Error(c, errcode.ErrParam, "无效的会话 ID")
		return
	}

	var req request.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, _ := getCurrentUserID(c)

	replay, ch, unsub, err := h.svc.StreamChat(c.Request.Context(), sessionID, req.Question, userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	writeStream(c, replay, ch, unsub)
}

// ResumeStream 续传进行中的生成（GET ?since=N）。
//
// GET /api/v1/portal/chat-sessions/:id/stream?since=N
func (h *ChatHandler) ResumeStream(c *gin.Context) {
	sessionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		resp.Error(c, errcode.ErrParam, "无效的会话 ID")
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
func (h *ChatHandler) CancelGeneration(c *gin.Context) {
	sessionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		resp.Error(c, errcode.ErrParam, "无效的会话 ID")
		return
	}
	userID, _ := getCurrentUserID(c)

	if err := h.svc.CancelGeneration(c.Request.Context(), sessionID, userID); err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, nil)
}

// writeStream 将历史回放 + 实时事件写入 SSE 客户端。
func writeStream(c *gin.Context, replay []StreamEvent, ch <-chan StreamEvent, unsub func()) {
	defer unsub()

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		resp.Error(c, errcode.ErrUnknown, "当前服务器不支持 SSE 流式输出")
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
	heartbeat := time.NewTicker(5 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprintf(c.Writer, ": heartbeat\n\n")
			flusher.Flush()
			rc.SetWriteDeadline(time.Now().Add(30 * time.Second))
		case evt, ok := <-ch:
			if !ok {
				return
			}
			_ = writeSSEEvent(c.Writer, evt)
			flusher.Flush()
			rc.SetWriteDeadline(time.Now().Add(30 * time.Second))
		}
	}
}
