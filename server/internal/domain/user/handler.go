// handler.go 合并认证、角色、用户三个 Handler 及其共享工具函数。
//
// Handler 层职责：参数校验、调用 Service、格式化响应，不包含业务规则。
// parseID / parsePagination / getCurrentUserID 为本领域 Handler 自用的本地副本，
// 与 handler/common.go 中的同名函数行为一致——领域包独立编译，不依赖 handler 包。
package user

import (
	"errors"
	"log/slog"
	"strconv"

	"opsmind/internal/shared/dto/request"
	"opsmind/internal/shared/pkg/errcode"
	"opsmind/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

// =============================================================================
// Handler 共享工具
// =============================================================================

// parsePagination 从查询参数中解析分页参数（page, pageSize）。
//
// 默认值：page=1, pageSize=10。上限：pageSize≤100。
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

// parseID 从路径参数中解析 int64 ID，解析失败时自动返回错误响应。
//
// 返回值 ok=false 表示解析失败，调用方应直接 return。
func parseID(c *gin.Context, key string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrParam, "无效的 "+key)
		return 0, false
	}
	return id, true
}

// getCurrentUserID 从 Gin context 中获取当前用户 ID。
//
// JWTAuth 中间件将当前用户 ID 以 int64 类型写入 context，key 为 "userID"。
func getCurrentUserID(c *gin.Context) (int64, bool) {
	if val, exists := c.Get("userID"); exists {
		if id, ok := val.(int64); ok {
			return id, true
		}
	}
	return 0, false
}

// handleServiceError 统一处理 Service 层错误。
//
// AppError 类型提取业务码，其他错误视为 500。
func handleServiceError(c *gin.Context, err error) {
	var appErr AppError
	if errors.As(err, &appErr) {
		response.Error(c, appErr.Code, appErr.Message)
		return
	}
	// 非 AppError 说明是未预期的内部错误，记录真实原因方便排查
	slog.Error("未预期的服务错误", "path", c.Request.URL.Path, "error", err)
	response.Error(c, errcode.ErrUnknown, "服务器内部错误")
}

// =============================================================================
// 认证 Handler
// =============================================================================

// AuthHandler 认证 Handler
type AuthHandler struct {
	authService *AuthService
}

// NewAuthHandler 创建 AuthHandler 实例
func NewAuthHandler(authService *AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Login 处理登录请求。
//
// POST /api/v1/auth/login
// 参数校验失败返回 400，业务错误返回对应错误码，成功返回 LoginResponse。
// 登录失败审计日志由 AuthService.Login 内部 slog 记录（不泄露用户名是否存在）。
func (h *AuthHandler) Login(c *gin.Context) {
	var req request.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// Refresh 处理刷新令牌请求。
//
// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req request.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	resp, err := h.authService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, resp)
}

// ChangePassword 处理修改密码请求。
//
// POST /api/v1/auth/me/change-password
// 从 JWT context 获取当前用户 ID（由认证中间件写入）。
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req request.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	// 从 context 获取用户 ID（JWT 中间件写入）
	userID, exists := c.Get("userID")
	if !exists {
		response.Error(c, errcode.ErrAuth, "未登录")
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		response.Error(c, errcode.ErrUnknown, "用户信息异常")
		return
	}
	err := h.authService.ChangePassword(c.Request.Context(), uid, req.OldPassword, req.NewPassword)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
}

// Logout 处理退出登录请求。
//
// POST /api/v1/auth/me/logout
// 将 refresh token 加入内存黑名单，阻止其被用于刷新。
func (h *AuthHandler) Logout(c *gin.Context) {
	var req request.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	if err := h.authService.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		handleServiceError(c, err)
		return
	}

	response.Success(c, nil)
}

// =============================================================================
// 角色 Handler
// =============================================================================

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

// =============================================================================
// 用户 Handler
// =============================================================================

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

// BatchDelete 批量删除用户。
//
// POST /api/v1/admin/users/batch-delete
func (h *UserHandler) BatchDelete(c *gin.Context) {
	var req request.BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}
	deleted, err := h.svc.BatchDelete(c.Request.Context(), req.IDs)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	response.Success(c, map[string]int64{"deleted": deleted})
}
