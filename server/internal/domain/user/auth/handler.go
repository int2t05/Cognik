// Package auth 认证 HTTP 请求处理。
package auth

import (
	"errors"
	"log/slog"
	"strconv"

	"cognik/internal/shared/dto/request"
	"cognik/internal/shared/pkg/errcode"
	resp "cognik/internal/shared/pkg/response"

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

// AuthHandler 认证 Handler。
type AuthHandler struct {
	authService *AuthService
}

// NewAuthHandler 创建 AuthHandler 实例。
func NewAuthHandler(authService *AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Login 处理登录请求。
//
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req request.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	result, err := h.authService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, result)
}

// Refresh 处理刷新令牌请求。
//
// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req request.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	result, err := h.authService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, result)
}

// ChangePassword 处理修改密码请求。
//
// POST /api/v1/auth/me/change-password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req request.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		resp.Error(c, errcode.ErrAuth, "未登录")
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		resp.Error(c, errcode.ErrUnknown, "用户信息异常")
		return
	}
	err := h.authService.ChangePassword(c.Request.Context(), uid, req.OldPassword, req.NewPassword)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, nil)
}

// Logout 处理退出登录请求。
//
// POST /api/v1/auth/me/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	var req request.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Error(c, errcode.ErrParam, "参数校验失败: "+err.Error())
		return
	}

	if err := h.authService.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		handleServiceError(c, err)
		return
	}

	resp.Success(c, nil)
}
