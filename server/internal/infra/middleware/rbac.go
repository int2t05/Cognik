// Package middleware 提供 Gin 中间件。
package middleware

import (
	"log/slog"
	"strings"

	"cognik/internal/shared/pkg/errcode"
	"cognik/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

// RequirePermission 返回检查用户是否拥有指定权限（任意一个即通过）的中间件。
func RequirePermission(permissions ...string) gin.HandlerFunc {
	// 空 permissions 恒定拒绝所有请求，可能为配置遗漏
	if len(permissions) == 0 {
		slog.Warn("RBAC 中间件注册时 permissions 为空，该路由将拒绝所有请求")
	}

	return func(c *gin.Context) {
		val, exists := c.Get("currentUser")
		if !exists {
			response.Error(c, errcode.ErrAuth, "未登录")
			c.Abort()
			return
		}

		currentUser, ok := val.(CurrentUser)
		if !ok {
			response.Error(c, errcode.ErrUnknown, "用户信息类型错误")
			c.Abort()
			return
		}

		if !hasAnyPermission(currentUser.Permissions, permissions) {
			response.Error(c, errcode.ErrForbidden, "无权限执行此操作")
			c.Abort()
			return
		}

		c.Next()
	}
}

// hasAnyPermission 检查用户权限列表中是否包含任意一个所需权限。
// 支持通配："*" 匹配一切，"prefix:*" 匹配同前缀权限。
func hasAnyPermission(userPerms []string, required []string) bool {
	if len(required) == 0 {
		return false // 安全默认值：无要求 = 谁都不可访问
	}
	if len(userPerms) == 0 {
		return false
	}

	// 精确匹配
	permSet := make(map[string]struct{}, len(userPerms))
	var wildcards []string // 通配前缀，不含尾随 "*"
	for _, p := range userPerms {
		if strings.HasSuffix(p, "*") {
			prefix := strings.TrimRight(p, "*")
			if prefix == "" {
				return true // "*" 匹配一切
			}
			wildcards = append(wildcards, prefix)
		} else {
			permSet[p] = struct{}{}
		}
	}

	for _, r := range required {
		if _, ok := permSet[r]; ok {
			return true
		}
		for _, prefix := range wildcards {
			if strings.HasPrefix(r, prefix) {
				return true
			}
		}
	}
	return false
}
