// Package middleware_test 验证 JWT 认证中间件。
//
// 测试覆盖 PLAN.md T12 定义的 4 个场景：
// 有效令牌、过期令牌、缺失 Authorization、格式错误。
package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"opsmind/internal/middleware"
	pkgjwt "opsmind/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test_jwt_secret_key"

// setupAuthRouter 创建带 JWT 中间件的测试路由。
//
// 受保护路由返回 currentUser 信息，用于验证中间件写入 context 的数据。
func setupAuthRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	protected := r.Group("/api/v1/protected")
	protected.Use(middleware.JWTAuth(nil, testSecret))
	protected.GET("/me", func(c *gin.Context) {
		user, exists := c.Get("currentUser")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "currentUser not set"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"user": user})
	})

	return r
}

// TestJWTAuth_ValidToken 有效令牌应通过中间件并写入 currentUser。
func TestJWTAuth_ValidToken(t *testing.T) {
	r := setupAuthRouter()

	token, err := pkgjwt.GenerateAccessToken(42, "testuser", []string{"admin"}, nil, testSecret, time.Hour)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/protected/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "有效令牌应通过认证")

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	user, ok := resp["user"].(map[string]interface{})
	require.True(t, ok, "currentUser 应为对象")
	assert.Equal(t, float64(42), user["user_id"])
	assert.Equal(t, "testuser", user["username"])
}

// TestJWTAuth_ExpiredToken 过期令牌应返回 401。
func TestJWTAuth_ExpiredToken(t *testing.T) {
	r := setupAuthRouter()

	// 生成已过期的令牌（-1 小时）
	token, err := pkgjwt.GenerateAccessToken(42, "testuser", []string{"admin"}, nil, testSecret, -1*time.Hour)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/protected/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "过期令牌应返回 401")

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(10001), resp["code"])
}

// TestJWTAuth_MissingAuthorization 缺失 Authorization 头应返回 401。
func TestJWTAuth_MissingAuthorization(t *testing.T) {
	r := setupAuthRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/protected/me", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "缺失 Authorization 应返回 401")

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(10001), resp["code"])
}

// TestJWTAuth_WrongFormat 无 Bearer 前缀应返回 401。
func TestJWTAuth_WrongFormat(t *testing.T) {
	r := setupAuthRouter()

	token, err := pkgjwt.GenerateAccessToken(42, "testuser", []string{"admin"}, nil, testSecret, time.Hour)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/protected/me", nil)
	req.Header.Set("Authorization", token) // 缺少 "Bearer " 前缀
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "无 Bearer 前缀应返回 401")
}

// TestJWTAuth_RefreshTokenRejected 验证刷新令牌被中间件拒绝。
//
// 双令牌安全模型要求 refresh token 只能用于 /auth/refresh 端点，
// 不能当作 access token 用于业务 API 认证。
func TestJWTAuth_RefreshTokenRejected(t *testing.T) {
	r := setupAuthRouter()

	token, err := pkgjwt.GenerateRefreshToken(42, "testuser", []string{"admin"}, nil, testSecret, 7*24*time.Hour)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/protected/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "refresh token 用于 API 认证应返回 401")

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(10001), resp["code"])
}

// TestJWTAuth_InvalidToken 无效令牌字符串应返回 401。
func TestJWTAuth_InvalidToken(t *testing.T) {
	r := setupAuthRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/protected/me", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "无效令牌应返回 401")
}

// TestJWTAuth_EmptySecret 验证空 secret 时中间件返回配置错误。
//
// 这是纵深防御的最后一道防线——main.go 应在启动阶段拒绝空 secret。
// 中间件层面返回 500（ErrUnknown），而非 401，原因是这是服务端配置问题而非客户端认证失败。
func TestJWTAuth_EmptySecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	protected := r.Group("/api/v1/protected")
	protected.Use(middleware.JWTAuth(nil, ""))
	protected.GET("/me", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/protected/me", nil)
	req.Header.Set("Authorization", "Bearer some.token.here")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code, "空 secret 应返回 500（服务端配置错误）")

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(99999), resp["code"], "业务码应为 99999（未知错误）")
}
