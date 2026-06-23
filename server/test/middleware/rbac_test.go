// Package middleware_test 验证 RBAC 权限中间件。
package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"opsmind/internal/middleware"

	"github.com/gin-gonic/gin"
)

// setupRBACRouter 创建带 RBAC 中间件的测试路由。
func setupRBACRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// 模拟 JWT 中间件注入 currentUser
	r.Use(func(c *gin.Context) {
		userJSON := c.GetHeader("X-Test-User")
		if userJSON != "" {
			var user middleware.CurrentUser
			json.Unmarshal([]byte(userJSON), &user)
			c.Set("currentUser", user)
			c.Set("userID", user.UserID)
		}
		c.Next()
	})

	admin := r.Group("/api/v1/admin")
	admin.Use(middleware.RequirePermission("ticket:read", "ticket:write"))
	admin.GET("/tickets", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
	})

	configGroup := r.Group("/api/v1/admin/config")
	configGroup.Use(middleware.RequirePermission("system:config"))
	configGroup.GET("", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
	})

	return r
}

func TestRBAC_UserWithPermission(t *testing.T) {
	r := setupRBACRouter()

	user := middleware.CurrentUser{
		UserID:      1,
		Username:    "admin",
		Roles:       []string{"系统管理员"},
		Permissions: []string{"ticket:read", "ticket:write", "system:config"},
	}
	userJSON, _ := json.Marshal(user)

	req := httptest.NewRequest("GET", "/api/v1/admin/tickets", nil)
	req.Header.Set("X-Test-User", string(userJSON))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望 200, got %d", w.Code)
	}
}

func TestRBAC_UserWithoutPermission(t *testing.T) {
	r := setupRBACRouter()

	user := middleware.CurrentUser{
		UserID:   2,
		Username: "reporter",
		Roles:    []string{"报障人"},
	}
	userJSON, _ := json.Marshal(user)

	req := httptest.NewRequest("GET", "/api/v1/admin/tickets", nil)
	req.Header.Set("X-Test-User", string(userJSON))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("期望 403, got %d", w.Code)
	}
}

func TestRBAC_NoCurrentUser(t *testing.T) {
	r := setupRBACRouter()

	req := httptest.NewRequest("GET", "/api/v1/admin/tickets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("期望 401, got %d", w.Code)
	}
}

// TestRBAC_WildcardPermission 验证 "*" 通配权限匹配任意所需权限。
func TestRBAC_WildcardPermission(t *testing.T) {
	r := setupRBACRouter()

	user := middleware.CurrentUser{
		UserID:      3,
		Username:    "superadmin",
		Roles:       []string{"系统管理员"},
		Permissions: []string{"*"},
	}
	userJSON, _ := json.Marshal(user)

	// 超级管理员应能访问所有路由
	for _, path := range []string{"/api/v1/admin/tickets", "/api/v1/admin/config"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			req.Header.Set("X-Test-User", string(userJSON))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("%s: 期望 200, got %d", path, w.Code)
			}
		})
	}
}

// TestRBAC_PrefixWildcard 验证 "prefix:*" 通配权限匹配同前缀的所需权限。
func TestRBAC_PrefixWildcard(t *testing.T) {
	r := setupRBACRouter()

	user := middleware.CurrentUser{
		UserID:      4,
		Username:    "ticket_manager",
		Roles:       []string{"运维人员"},
		Permissions: []string{"ticket:*"},
	}
	userJSON, _ := json.Marshal(user)

	// ticket:* 应能访问 ticket:read / ticket:write 保护的路由
	req := httptest.NewRequest("GET", "/api/v1/admin/tickets", nil)
	req.Header.Set("X-Test-User", string(userJSON))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ticket:* 应能匹配 ticket:read / ticket:write，期望 200, got %d", w.Code)
	}

	// ticket:* 不应能访问 system:config 保护的路由
	req2 := httptest.NewRequest("GET", "/api/v1/admin/config", nil)
	req2.Header.Set("X-Test-User", string(userJSON))
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusForbidden {
		t.Errorf("ticket:* 不应匹配 system:config，期望 403, got %d", w2.Code)
	}
}

// TestRBAC_PartialMatch 验证用户拥有部分（非全部）所需权限时仍可通过。
//
// RBAC 中间件采用"任意一个满足即可"策略，而非"全部满足"。
func TestRBAC_PartialMatch(t *testing.T) {
	r := setupRBACRouter()

	// 用户只有 ticket:read，但路由要求 ticket:read 或 ticket:write（任意一个）
	user := middleware.CurrentUser{
		UserID:      5,
		Username:    "readonly",
		Roles:       []string{"运维人员"},
		Permissions: []string{"ticket:read"},
	}
	userJSON, _ := json.Marshal(user)

	req := httptest.NewRequest("GET", "/api/v1/admin/tickets", nil)
	req.Header.Set("X-Test-User", string(userJSON))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("仅有 ticket:read 也应通过要求 ticket:read|ticket:write 的路由，期望 200, got %d", w.Code)
	}
}
