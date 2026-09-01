// Package middleware 实现 Gin HTTP 中间件.
//
// rbac.go 提供 RBAC 权限校验中间件。
// 设计决策：采用 RequirePermission(permissions ...string) 形式而非 RequireRole，
// 原因是权限比角色更稳定（角色名可能变，但 ticket:read 语义不变），
// 且与 PRD 中"菜单/按钮级权限"的需求对齐。
package middleware

import (
	"log/slog"
	"strings"

	"opsmind/internal/shared/pkg/errcode"
	"opsmind/internal/shared/pkg/response"

	"github.com/gin-gonic/gin"
)

// RequirePermission 返回检查用户是否拥有指定权限的中间件。
//
// 权限判断逻辑：检查 currentUser.Permissions 中是否包含任意一个所需权限。
// 为什么用"任意一个"而非"全部"：大多数场景下接口只需一个权限即可访问，
// 如需同时满足多个权限，可在业务层做更细粒度的校验。
func RequirePermission(permissions ...string) gin.HandlerFunc {
	// 启动时告警：空 permissions 会恒定拒绝所有请求，可能为路由配置遗漏。
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
//
// 支持两种通配模式：
//   - "*" 匹配任意权限（等效超级管理员）
//   - "knowledge:*" 匹配 knowledge:read、knowledge:write 等同前缀权限
//
// 通配匹配发生在精确匹配之后（精确匹配优先），使用前缀而非正则避免性能开销。
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
