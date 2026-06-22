// Package handler 实现 HTTP 请求处理。
//
// user.go 提供用户管理相关接口。
// Handler 层职责：参数解析、调用 Service、格式化响应。
// 不包含业务逻辑，所有校验和状态机在 Service 层完成。
package handler

import (
	"opsmind/internal/dto/request"
	"opsmind/internal/service"
	"opsmind/pkg/errcode"
	"opsmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// UserHandler 用户管理接口。
type UserHandler struct {
	svc *service.UserService
}

// NewUserHandler 创建 UserHandler 实例。
func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// Create 创建用户。
//
// POST /api/v1/admin/users
func (h *UserHandler) Create(c *gin.Context) {
	var req request.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	if svcErr := h.svc.Create(c.Request.Context(), req); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
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

	response.Success(c, user)
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

	response.SuccessWithPage(c, result.Users, result.Total, page, pageSize)
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
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	if svcErr := h.svc.Update(c.Request.Context(), id, req); svcErr != nil {
		handleServiceError(c, svcErr)
		return
	}

	response.Success(c, nil)
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

	response.Success(c, nil)
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

	response.Success(c, nil)
}
