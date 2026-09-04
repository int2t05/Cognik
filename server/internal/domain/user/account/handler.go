// Package account 用户账户 HTTP 请求处理。
package account

import (
	"errors"
	"log/slog"
	"strconv"

	"cognos/internal/shared/dto/request"
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

// UserHandler 用户管理接口。
type UserHandler struct {
	svc *UserService
}

// NewUserHandler 创建 UserHandler 实例。
func NewUserHandler(svc *UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// Create 创建用户。
//
// POST /api/v1/admin/users
func (h *UserHandler) Create(c *gin.Context) {
	var req request.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	if svcErr := h.svc.Create(c.Request.Context(), req); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	resp.Success(c, nil)
}

// GetByID 获取用户详情。
//
// GET /api/v1/admin/users/:id
func (h *UserHandler) GetByID(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	user, svcErr := h.svc.GetByID(c.Request.Context(), id)
	if svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	resp.Success(c, user)
}

// List 用户列表（分页）。
//
// GET /api/v1/admin/users
func (h *UserHandler) List(c *gin.Context) {
	page, pageSize := parsePagination(c)
	keyword := c.Query("keyword")

	result, err := h.svc.List(c.Request.Context(), page, pageSize, keyword)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	resp.SuccessWithPage(c, result.Users, result.Total, page, pageSize)
}

// Update 更新用户基本信息。
//
// PUT /api/v1/admin/users/:id
func (h *UserHandler) Update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req request.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	if svcErr := h.svc.Update(c.Request.Context(), id, req); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	resp.Success(c, nil)
}

// Freeze 冻结用户。
//
// PATCH /api/v1/admin/users/:id/freeze
func (h *UserHandler) Freeze(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	operatorID, _ := getCurrentUserID(c)

	if svcErr := h.svc.Freeze(c.Request.Context(), id, operatorID); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	resp.Success(c, nil)
}

// Restore 恢复已冻结用户。
//
// PATCH /api/v1/admin/users/:id/unfreeze
func (h *UserHandler) Restore(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	if svcErr := h.svc.Restore(c.Request.Context(), id); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	resp.Success(c, nil)
}

// BatchDelete 批量删除用户。
//
// POST /api/v1/admin/users/batch-delete
func (h *UserHandler) BatchDelete(c *gin.Context) {
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
