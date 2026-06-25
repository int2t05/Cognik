// Package handler 实现 HTTP 请求处理。
//
// message.go 提供站内消息相关接口。
// Handler 层职责：参数解析、调用 Service、格式化响应。
package handler

import (
	"strconv"

	"opsmind/internal/repository"
	"opsmind/internal/service"
	"opsmind/pkg/errcode"
	"opsmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// MessageHandler 站内消息接口。
type MessageHandler struct {
	svc *service.MessageService
}

// NewMessageHandler 创建 MessageHandler 实例。
func NewMessageHandler(svc *service.MessageService) *MessageHandler {
	return &MessageHandler{svc: svc}
}

// =============================================================================
// 门户端
// =============================================================================

// ListMessages 查询当前用户的消息列表，支持 is_read/type 过滤。
//
// GET /api/v1/portal/messages?is_read=true&type=ticket_supplement
func (h *MessageHandler) ListMessages(c *gin.Context) {
	userID, _ := getCurrentUserID(c)

	page, pageSize := parsePagination(c)

	// 解析可选过滤参数
	var filter repository.MessageFilter
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

	response.SuccessWithPage(c, msgs, total, page, pageSize)
}

// MarkAsRead 标记消息为已读。
//
// PUT /api/v1/portal/messages/:id/read
// 校验消息归属（currentUserID），防止水平越权。
func (h *MessageHandler) MarkAsRead(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的消息 ID")
		return
	}

	userID, _ := getCurrentUserID(c)
	count, err := h.svc.MarkAsReadAndCount(c.Request.Context(), id, userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, gin.H{"unread_count": count})
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
	response.Success(c, gin.H{"affected": affected})
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

	response.Success(c, map[string]int64{"count": count})
}
