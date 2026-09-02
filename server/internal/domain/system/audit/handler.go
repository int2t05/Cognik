// Package audit 审计日志 HTTP 请求处理。
package audit

import (
	"errors"
	"log/slog"
	"strconv"

	"opsmind/internal/shared/dto/request"
	"opsmind/internal/shared/pkg/errcode"
	resp "opsmind/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

// parsePagination 解析分页参数（page 默认 1，pageSize 默认 10，上限 100）。
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

// AuditHandler 审计日志查询接口。
type AuditHandler struct {
	svc *AuditService
}

// NewAuditHandler 创建 AuditHandler 实例。
func NewAuditHandler(svc *AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// List 查询审计日志列表（支持多维过滤和日期范围）。
//
// GET /api/v1/admin/audit-logs
func (h *AuditHandler) List(c *gin.Context) {
	var req request.AuditLogListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	page, pageSize := parsePagination(c)

	f := AuditFilter{
		OperatorID: req.OperatorID,
		Action:     req.Action,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		DateFrom:   req.DateFrom,
		DateTo:     req.DateTo,
		Page:       page,
		PageSize:   pageSize,
	}

	items, total, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	resp.SuccessWithPage(c, items, total, page, pageSize)
}

// BatchDelete 批量删除审计日志。
//
// POST /api/v1/admin/audit-logs/batch-delete
func (h *AuditHandler) BatchDelete(c *gin.Context) {
	var req request.BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}
	deleted, err := h.svc.BatchDelete(c.Request.Context(), req.IDs)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, map[string]int64{"deleted": deleted})
}
