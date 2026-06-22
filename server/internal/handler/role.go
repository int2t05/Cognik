// Package handler 实现 HTTP 请求处理。
//
// role.go 提供角色管理相关接口。
// Handler 层职责：参数解析、调用 Service、格式化响应。
package handler

import (
	"opsmind/internal/dto/request"
	"opsmind/internal/service"
	"opsmind/pkg/errcode"
	"opsmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// RoleHandler 角色管理接口。
type RoleHandler struct {
	svc *service.RoleService
}

// NewRoleHandler 创建 RoleHandler 实例。
func NewRoleHandler(svc *service.RoleService) *RoleHandler {
	return &RoleHandler{svc: svc}
}

// Create 创建角色。
func (h *RoleHandler) Create(c *gin.Context) {
	var req request.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	if err := h.svc.Create(c.Request.Context(), req.Name, req.Description, req.Permissions); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
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

	response.Success(c, role)
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

	response.SuccessWithPage(c, roles, total, page, pageSize)
}

// Update 更新角色。
func (h *RoleHandler) Update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	var req request.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	if err := h.svc.Update(c.Request.Context(), id, req.Name, req.Description, req.Permissions); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
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

	response.Success(c, nil)
}

// =============================================================================
// 菜单管理
// =============================================================================

// ListMenus 获取全部菜单列表。
//
// GET /api/v1/admin/menus
func (h *RoleHandler) ListMenus(c *gin.Context) {
	menus, err := h.svc.ListMenus(c.Request.Context())
	if err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, menus)
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
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	if err := h.svc.UpdateRoleMenus(c.Request.Context(), id, body.MenuIDs); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
}
