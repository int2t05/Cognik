package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"opsmind/internal/middleware"
)

func setupRequestIDRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.GET("/test", func(c *gin.Context) {
		rid, _ := c.Get(middleware.RequestIDKey)
		c.JSON(http.StatusOK, gin.H{"request_id": rid})
	})
	return r
}

// TestRequestID_Generated 验证未传入 X-Request-ID 时自动生成 UUID
func TestRequestID_Generated(t *testing.T) {
	r := setupRequestIDRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 %d", w.Code, http.StatusOK)
	}

	rid := w.Header().Get(middleware.RequestIDKey)
	if rid == "" {
		t.Fatal("响应头 X-Request-ID 为空，期望自动生成")
	}
	// UUID v4 格式：8-4-4-4-12
	if len(rid) != 36 {
		t.Errorf("X-Request-ID 长度 = %d, 期望 36 (UUID 格式): %s", len(rid), rid)
	}
}

// TestRequestID_Passthrough 验证客户端传入的 X-Request-ID 被复用
func TestRequestID_Passthrough(t *testing.T) {
	r := setupRequestIDRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(middleware.RequestIDKey, "custom-id-123")
	r.ServeHTTP(w, req)

	rid := w.Header().Get(middleware.RequestIDKey)
	if rid != "custom-id-123" {
		t.Errorf("X-Request-ID = %q, 期望 custom-id-123 (客户端传入应被复用)", rid)
	}
}

// TestRequestID_ContextValue 验证请求 ID 被写入 Gin Context
func TestRequestID_ContextValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID())

	var captured string
	r.GET("/test", func(c *gin.Context) {
		captured = c.GetString(middleware.RequestIDKey)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	if captured == "" {
		t.Error("Gin Context 中 X-Request-ID 为空，期望非空")
	}
}
