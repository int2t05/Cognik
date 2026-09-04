// Package message 站内消息 HTTP 请求处理。
package message

import (
	"errors"
	"log/slog"
	"strconv"

	respDto "cognos/internal/shared/dto/response"
	"cognos/internal/shared/pkg/errcode"
	resp "cognos/internal/shared/pkg/response"

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

// MessageHandler 站内消息接口。
type MessageHandler struct {
	svc *MessageService
}

// NewMessageHandler 创建 MessageHandler 实例。
func NewMessageHandler(svc *MessageService) *MessageHandler {
	return &MessageHandler{svc: svc}
}

// ListMessages 查询当前用户的消息列表，支持 is_read/type 过滤。
//
// GET /api/v1/portal/messages
func (h *MessageHandler) ListMessages(c *gin.Context) {
	userID, _ := getCurrentUserID(c)

	page, pageSize := parsePagination(c)

	var filter MessageFilter
	if v := c.Query("is_read"); v != "" {
		b := v == "true" || v == "1"
		filter.IsRead = &b
	}
	filter.Type = c.Query("type")

	msgs, total, err := h.svc.ListMessages(c.Request.Context(), userID, page, pageSize, filter)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	resp.SuccessWithPage(c, msgs, total, page, pageSize)
}

// MarkAsRead 标记消息为已读。
//
// PUT /api/v1/portal/messages/:id/read
func (h *MessageHandler) MarkAsRead(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		resp.Error(c, errcode.ErrParam, "无效的消息 ID")
		return
	}

	userID, _ := getCurrentUserID(c)
	count, err := h.svc.MarkAsReadAndCount(c.Request.Context(), id, userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, respDto.MarkAsReadResponse{UnreadCount: count})
}

// MarkAllRead 标记当前用户所有消息为已读。
//
// PUT /api/v1/portal/messages/read-all
func (h *MessageHandler) MarkAllRead(c *gin.Context) {
	userID, _ := getCurrentUserID(c)
	affected, err := h.svc.MarkAllRead(c.Request.Context(), userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, respDto.MarkAllReadResponse{Affected: affected})
}

// CountUnread 查询未读消息数。
//
// GET /api/v1/portal/messages/unread-count
func (h *MessageHandler) CountUnread(c *gin.Context) {
	userID, _ := getCurrentUserID(c)

	count, err := h.svc.CountUnread(c.Request.Context(), userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, respDto.UnreadCountResponse{Count: count})
}
