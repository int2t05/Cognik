// Package session Agent 对话 HTTP 请求处理。
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"opsmind/internal/shared/pkg/errcode"
	resp "opsmind/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

// getCurrentUserID 从 Gin context 获取当前用户 ID。
func getCurrentUserID(c *gin.Context) (int64, bool) {
	if val, exists := c.Get("userID"); exists {
		if id, ok := val.(int64); ok {
			return id, true
		}
	}
	return 0, false
}

// parseID 从路径参数解析 int64 ID。
func parseID(c *gin.Context, key string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil {
		resp.Error(c, errcode.ErrParam, "无效的 "+key)
		return 0, false
	}
	return id, true
}

// handleServiceError 统一处理 Service 错误。
func handleServiceError(c *gin.Context, err error) {
	var appErr errcode.AppError
	if errors.As(err, &appErr) {
		resp.Error(c, appErr.Code, appErr.Message)
		return
	}
	resp.Error(c, errcode.ErrUnknown, "服务器内部错误")
}

// ChatHandler Agent 对话接口。
type ChatHandler struct {
	svc *ChatService
}

// NewChatHandler 创建 ChatHandler。
func NewChatHandler(svc *ChatService) *ChatHandler {
	return &ChatHandler{svc: svc}
}

// CreateThread 创建对话线程。
// POST /api/v1/portal/threads
func (h *ChatHandler) CreateThread(c *gin.Context) {
	var req struct {
		Title string `json:"title"`
	}
	_ = c.ShouldBindJSON(&req)

	userID, _ := getCurrentUserID(c)
	thread, err := h.svc.CreateThread(c.Request.Context(), userID, req.Title)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, thread)
}

// ListThreads 列出用户的对话线程。
// GET /api/v1/portal/threads
func (h *ChatHandler) ListThreads(c *gin.Context) {
	userID, _ := getCurrentUserID(c)
	threads, err := h.svc.ListThreads(c.Request.Context(), userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, threads)
}

// GetThreadDetail 获取线程详情（含消息）。
// GET /api/v1/portal/threads/:id
func (h *ChatHandler) GetThreadDetail(c *gin.Context) {
	userID, _ := getCurrentUserID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	detail, err := h.svc.GetThreadDetail(c.Request.Context(), id, userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, detail)
}

// DeleteThread 删除对话线程。
// DELETE /api/v1/portal/threads/:id
func (h *ChatHandler) DeleteThread(c *gin.Context) {
	userID, _ := getCurrentUserID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteThread(c.Request.Context(), id, userID); err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, nil)
}

// UpdateThread 更新线程标题。
// PATCH /api/v1/portal/threads/:id
func (h *ChatHandler) UpdateThread(c *gin.Context) {
	userID, _ := getCurrentUserID(c)
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败")
		return
	}
	if err := h.svc.UpdateThread(c.Request.Context(), id, userID, req.Title); err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, nil)
}

// writeSSEEvent 将事件序列化为 SSE 帧：id: {seq}\ndata: {json}\n\n
// id 字段供 SSE 标准 Last-Event-ID 重连。
func writeSSEEvent(w gin.ResponseWriter, evt StreamEvent) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %d\ndata: %s\n\n", evt.Seq, string(data))
	return err
}

// StreamChat 发送消息并以 SSE 流式返回 Agent 回答。
// POST /api/v1/portal/threads/:id/stream
func (h *ChatHandler) StreamChat(c *gin.Context) {
	threadID, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Question string `json:"question" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}
	userID, _ := getCurrentUserID(c)

	replay, ch, unsub, err := h.svc.StreamChat(c.Request.Context(), threadID, req.Question, userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	writeStream(c, replay, ch, unsub)
}

// ResumeStream 续传进行中的生成（GET ?since=N）。
// GET /api/v1/portal/threads/:id/stream?since=N
func (h *ChatHandler) ResumeStream(c *gin.Context) {
	threadID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, _ := getCurrentUserID(c)
	since, _ := strconv.Atoi(c.DefaultQuery("since", "0"))

	replay, ch, unsub, err := h.svc.ResumeStream(c.Request.Context(), threadID, userID, since)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	writeStream(c, replay, ch, unsub)
}

// CancelGeneration 取消生成。
// POST /api/v1/portal/threads/:id/cancel
func (h *ChatHandler) CancelGeneration(c *gin.Context) {
	threadID, ok := parseID(c, "id")
	if !ok {
		return
	}
	userID, _ := getCurrentUserID(c)
	if err := h.svc.CancelGeneration(c.Request.Context(), threadID, userID); err != nil {
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