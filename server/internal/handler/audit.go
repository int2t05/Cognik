// Package handler 实现 HTTP 请求处理。
//
// audit.go 提供审计日志查询接口。
// 审计日志写入由各 Service 层直接调用 AuditRepo.Create，不经过 Handler。
package handler

import (
	"opsmind/internal/dto/request"
	"opsmind/internal/repository"
	"opsmind/internal/service"
	"opsmind/pkg/errcode"
	"opsmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// AuditHandler 审计日志查询接口。
type AuditHandler struct {
	svc *service.AuditService
}

// NewAuditHandler 创建 AuditHandler 实例。
func NewAuditHandler(svc *service.AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// List 查询审计日志列表（支持多维过滤和日期范围）。
//
// GET /api/v1/admin/audit-logs?operator_id=1&action=user.create&target_type=user&target_id=42&date_from=2026-01-01&date_to=2026-06-30
func (h *AuditHandler) List(c *gin.Context) {
	var req request.AuditLogListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	page, pageSize := parsePagination(c)

	f := repository.AuditFilter{
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

	response.SuccessWithPage(c, items, total, page, pageSize)
}
