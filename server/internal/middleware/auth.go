// Package middleware 提供 Gin 中间件。
//
// 本文件实现 JWT 认证中间件，从 Authorization 头提取 Bearer 令牌，
// 解析后将用户信息写入 Gin context，供下游 Handler 使用。
package middleware

import (
	"context"
	"strings"

	"opsmind/internal/cache"
	"opsmind/pkg/errcode"
	"opsmind/pkg/jwt"
	"opsmind/pkg/response"

	"github.com/gin-gonic/gin"
)

// CurrentUser JWT 解析后的用户信息，写入 Gin context。
//
// Permissions 由 JWT 中间件根据角色映射填充，RBAC 中间件使用该字段做权限校验。
type CurrentUser struct {
	UserID      int64    `json:"user_id"`
	Username    string   `json:"username"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// 权限解析从 JWT Claims.Permissions 读取，不再使用硬编码映射。
// 权限在登录时由 AuthService 从 Role.Permissions（数据库 JSONB 字段）解析后写入 JWT，
// 中间件只需从 Claims 中读取即可，新增角色/修改权限无需改代码。

// JWTAuth 返回 JWT 认证中间件。
//
// userCache 用于校验用户状态（冻结/存在性），内存缓存减少 DB 查询。
// 测试环境传 nil 跳过 DB 校验。secret 为空时返回配置错误。
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

		// 校验用户状态——优先内存缓存，未命中回退 DB
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
