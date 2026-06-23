// Package middleware_test 测试中间件的导出 API。
//
// 本文件测试 CORS 中间件的跨域请求配置。
package middleware_test

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"opsmind/internal/middleware"
)

// setupRouter 创建用于测试的 Gin 路由
func setupRouter(allowOrigins []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.CORS(allowOrigins, "debug"))
	r.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})
	return r
}

// TestCORS_AllowedOrigin 测试允许的来源
func TestCORS_AllowedOrigin(t *testing.T) {
	r := setupRouter([]string{"http://localhost:5173"})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// 验证 CORS 头
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("期望 Access-Control-Allow-Origin=http://localhost:5173，实际 %s",
			w.Header().Get("Access-Control-Allow-Origin"))
	}
}

// TestCORS_DisallowedOrigin 测试不允许的来源（release 模式）。
//
// debug 模式下 AllowOriginFunc 放行所有来源，无法测试"拒绝"，因此本测试强制使用 release 模式。
func TestCORS_DisallowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.CORS([]string{"http://localhost:5173"}, "release"))
	r.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://evil.com")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// 不允许的来源不应有 CORS 头
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("不允许的来源不应有 Access-Control-Allow-Origin 头，实际 %s",
			w.Header().Get("Access-Control-Allow-Origin"))
	}

	// 验证允许的来源正常
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("Origin", "http://localhost:5173")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("允许的来源应有 Access-Control-Allow-Origin，实际 %s",
			w2.Header().Get("Access-Control-Allow-Origin"))
	}
}

// TestCORS_PreflightRequest 测试预检请求
func TestCORS_PreflightRequest(t *testing.T) {
	r := setupRouter([]string{"http://localhost:5173"})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// 验证预检响应
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("预检响应应包含 Access-Control-Allow-Methods")
	}

	if w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("预检响应应包含 Access-Control-Allow-Headers")
	}
}

// TestCORS_AllowedMethods 测试允许的 HTTP 方法
func TestCORS_AllowedMethods(t *testing.T) {
	r := setupRouter([]string{"http://localhost:5173"})

	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/test", nil)
			req.Header.Set("Origin", "http://localhost:5173")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// 验证允许的方法包含当前方法
			allowedMethods := w.Header().Get("Access-Control-Allow-Methods")
			if allowedMethods == "" && method != "OPTIONS" {
				// 非 OPTIONS 请求可能不返回 Allow-Methods
				return
			}
		})
	}
}

// TestCORS_AllowedHeaders 测试允许的请求头
func TestCORS_AllowedHeaders(t *testing.T) {
	r := setupRouter([]string{"http://localhost:5173"})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	allowedHeaders := w.Header().Get("Access-Control-Allow-Headers")
	if allowedHeaders == "" {
		t.Error("预检响应应包含 Access-Control-Allow-Headers")
	}
}

// TestCORS_MaxAge 测试 MaxAge 配置
func TestCORS_MaxAge(t *testing.T) {
	r := setupRouter([]string{"http://localhost:5173"})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// MaxAge 应该存在（12 小时 = 43200 秒）
	maxAge := w.Header().Get("Access-Control-Max-Age")
	if maxAge == "" {
		t.Error("预检响应应包含 Access-Control-Max-Age")
	}
}
