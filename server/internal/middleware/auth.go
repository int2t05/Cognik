// Package middleware 提供 Gin 中间件。
//
// 本文件实现 JWT 认证中间件，从 Authorization 头提取 Bearer 令牌，
// 解析后将用户信息写入 Gin context，供下游 Handler 使用。
package middleware

import (
	"strings"

	"opsmind/pkg/errcode"
	"opsmind/pkg/jwt"
	"opsmind/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
// db 用于校验用户状态（冻结/存在性）；测试环境中可传 nil 跳过 DB 校验。
// secret 为空时中间件对所有请求返回配置错误，而非静默放行。
func JWTAuth(db *gorm.DB, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 纵深防御：secret 为空时应由 main.go 在启动阶段拒绝，
		// 此处作为运行时兜底，避免空密钥令牌被意外接受。
		if secret == "" {
			abortWithError(c, errcode.ErrUnknown, "JWT 密钥未配置")
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			abortWithError(c, errcode.ErrAuth, "缺失 Authorization 头")
			return
		}

		// 提取 Bearer 令牌
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

		// 拒绝 refresh token 用于 API 认证，保证双令牌安全模型
		if claims.TokenType != "access" {
			abortWithError(c, errcode.ErrAuth, "令牌类型错误，请使用访问令牌")
			return
		}

		// DB 可用时校验用户状态（冻结/存在性），
		// 弥补 JWT 纯签名校验的不足——token 签发后被冻结的用户应被即时拒绝。
		//
		// TODO(middleware/auth): 每次 API 请求都查 DB 获取用户状态，高并发时产生 N 次查询。
		// 可考虑 Redis 缓存用户状态（冻结/正常），token 签发时写入、变更时失效。
		if db != nil {
			var userStatus int
			// 仅查询 status 字段，不读取全表数据
			err := db.Table("users").
				Where("id = ?", claims.UserID).
				Select("status").
				Scan(&userStatus).Error
			if err != nil {
				abortWithError(c, errcode.ErrAuth, "用户不存在或已被删除")
				return
			}
			if userStatus == 2 {
				abortWithError(c, errcode.ErrAuth, "账号已被冻结")
				return
			}
		}

		// 从 JWT Claims 读取权限（登录时已从 Role.Permissions DB 字段解析）
		permissions := claims.Permissions
		if permissions == nil {
			permissions = []string{}
		}

		// 写入 context，供下游 Handler 使用
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
