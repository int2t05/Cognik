// Package dashboard 封装数据看板领域的 HTTP 请求处理层。
//
// handler.go 处理统计数据与趋势数据查询请求。
package dashboard

import (
	"errors"
	"log/slog"
	"strconv"

	"opsmind/internal/shared/dto/request"
	"opsmind/internal/shared/pkg/errcode"
	resp "opsmind/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

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

// handleServiceError 统一处理 Service 层错误。
func handleServiceError(c *gin.Context, err error) {
	var appErr errcode.AppError
	if errors.As(err, &appErr) {
		resp.Error(c, appErr.Code, appErr.Message)
		return
	}
	slog.Error("未预期的服务错误", "path", c.Request.URL.Path, "error", err)
	resp.Error(c, errcode.ErrUnknown, "服务器内部错误")
}

// DashboardHandler 数据看板接口。
type DashboardHandler struct {
	svc *DashboardService
}

// NewDashboardHandler 创建 DashboardHandler 实例。
func NewDashboardHandler(svc *DashboardService) *DashboardHandler {
	return &DashboardHandler{svc: svc}
}

// GetStats 获取看板统计数据。
//
// GET /api/v1/admin/dashboard/stats
func (h *DashboardHandler) GetStats(c *gin.Context) {
	result, err := h.svc.GetStats(c.Request.Context())
	if err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, result)
}

// GetTrends 获取趋势数据。
//
// GET /api/v1/admin/dashboard/trends
func (h *DashboardHandler) GetTrends(c *gin.Context) {
	var req request.TrendRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	result, err := h.svc.GetTrends(c.Request.Context(), req)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, result)
}
