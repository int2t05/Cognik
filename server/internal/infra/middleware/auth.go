// Package middleware 提供 Gin 中间件（JWT 认证、RBAC、CORS、请求日志、RequestID）。
package middleware

import (
	"context"
	"strings"

	"cognos/internal/infra/cache"
	"cognos/internal/shared/pkg/errcode"
	"cognos/internal/shared/pkg/jwt"
	"cognos/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

// CurrentUser JWT 解析后的用户信息，写入 Gin context 供下游使用。
type CurrentUser struct {
	UserID      int64    `json:"user_id"`
	Username    string   `json:"username"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// JWTAuth 返回 JWT 认证中间件。
// userCache 校验用户状态（冻结/存在性），测试环境传 nil 跳过。
func JWTAuth(userCache *cache.UserStatusCache, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" {
			abortWithError(c, errcode.ErrUnknown, "JWT 密钥未配置")
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			abortWithError(c, errcode.ErrAuth, "缺失 Authorization 头")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			abortWithError(c, errcode.ErrAuth, "Authorization 格式错误，应为 Bearer <token>")
			return
		}

		claims, err := jwt.ParseToken(parts[1], secret)
		if err != nil {
			abortWithError(c, errcode.ErrAuth, "令牌无效或已过期")
			return
		}

		if claims.TokenType != "access" {
			abortWithError(c, errcode.ErrAuth, "令牌类型错误，请使用访问令牌")
			return
		}

		// 校验用户状态，优先缓存未命中回退 DB
		if userCache != nil {
			status, err := userCache.GetStatus(context.Background(), claims.UserID)
			if err != nil {
				abortWithError(c, errcode.ErrAuth, "用户不存在或已被删除")
				return
			}
			if status == 2 {
				abortWithError(c, errcode.ErrAuth, "账号已被冻结")
				return
			}
		}

		permissions := claims.Permissions
		if permissions == nil {
			permissions = []string{}
		}

		c.Set("currentUser", CurrentUser{
			UserID:      claims.UserID,
			Username:    claims.Username,
			Roles:       claims.Roles,
			Permissions: permissions,
		})
		c.Set("userID", claims.UserID)

		c.Next()
	}
}

// abortWithError 中断请求并返回统一错误响应。
func abortWithError(c *gin.Context, code int, msg string) {
	response.Error(c, code, msg)
	c.Abort()
}
