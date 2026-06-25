// Package handler 实现认证相关的 HTTP Handler。
//
// AuthHandler 绑定 Gin HandlerFunc，负责参数校验和响应格式化。
// 业务逻辑委托给 AuthService，Handler 本身不包含业务规则。
package handler

import (
	"errors"
	"log/slog"

	"opsmind/internal/dto/request"
	"opsmind/internal/service"
	"opsmind/pkg/errcode"
	"opsmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// AuthHandler 认证 Handler
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler 创建 AuthHandler 实例
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
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

// handleServiceError 统一处理 Service 层错误。
//
// 为什么提取为独立函数：Login/Refresh/ChangePassword 共用相同的错误处理逻辑。
// AppError 类型提取业务码，其他错误视为 500。
func handleServiceError(c *gin.Context, err error) {
	var appErr service.AppError
	if errors.As(err, &appErr) {
		response.Error(c, appErr.Code, appErr.Message)
		return
	}
	// 非 AppError 说明是未预期的内部错误，记录真实原因方便排查
	slog.Error("未预期的服务错误", "path", c.Request.URL.Path, "error", err)
	response.Error(c, errcode.ErrUnknown, "服务器内部错误")
}
