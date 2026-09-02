// Package role 角色权限 HTTP 请求处理。
package role

import (
	"errors"
	"log/slog"
	"strconv"

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

// RoleHandler 角色管理接口。
type RoleHandler struct {
	svc *RoleService
}

// NewRoleHandler 创建 RoleHandler 实例。
func NewRoleHandler(svc *RoleService) *RoleHandler {
	return &RoleHandler{svc: svc}
}

// Create 创建角色。
func (h *RoleHandler) Create(c *gin.Context) {
	var req request.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	if err := h.svc.Create(c.Request.Context(), req.Name, req.Description, req.Permissions); err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, nil)
}

// GetByID 获取角色详情。
func (h *RoleHandler) GetByID(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	role, svcErr := h.svc.GetByID(c.Request.Context(), id)
	if svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	resp.Success(c, role)
}

// List 角色列表（分页 + 关键词搜索）。
func (h *RoleHandler) List(c *gin.Context) {
	page, pageSize := parsePagination(c)
	keyword := c.Query("keyword")

	roles, total, err := h.svc.List(c.Request.Context(), page, pageSize, keyword)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	resp.SuccessWithPage(c, roles, total, page, pageSize)
}

// Update 更新角色。
func (h *RoleHandler) Update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req request.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	if err := h.svc.Update(c.Request.Context(), id, req.Name, req.Description, req.Permissions); err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, nil)
}

// Delete 删除角色。
func (h *RoleHandler) Delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, nil)
}

// ListMenus 获取全部菜单列表。
//
// GET /api/v1/admin/menus
func (h *RoleHandler) ListMenus(c *gin.Context) {
	menus, err := h.svc.ListMenus(c.Request.Context())
	if err != nil {
		handleServiceError(c, err)
		return
	}
	resp.Success(c, menus)
}

// UpdateRoleMenus 更新角色菜单权限绑定。
//
// PUT /api/v1/admin/roles/:id/menus
func (h *RoleHandler) UpdateRoleMenus(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	var body struct {
		MenuIDs []int64 `json:"menu_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	if err := h.svc.UpdateRoleMenus(c.Request.Context(), id, body.MenuIDs); err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, nil)
}
