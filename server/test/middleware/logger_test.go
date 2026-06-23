// Package middleware_test 测试中间件的导出 API。
//
// 本文件测试请求日志中间件的 slog 输出。
package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"opsmind/internal/middleware"
)

// setupLoggerTest 创建带 Logger 中间件的测试路由，替换 slog 默认 handler 为写入 buf。
// 返回 restore 函数，测试结束时调用以还原 slog 默认 handler。
func setupLoggerTest(buf *bytes.Buffer) (*gin.Engine, func()) {
	// 保存旧 handler
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo})))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	r.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})
	r.POST("/test", func(c *gin.Context) {
		c.String(201, "created")
	})

	return r, func() { slog.SetDefault(old) }
}

// parseLog 解析 buf 中的 JSON 日志行，返回第一条匹配的日志条目。
func parseLog(t *testing.T, buf *bytes.Buffer) map[string]interface{} {
	t.Helper()
	// slog.NewJSONHandler 每行一条 JSON 记录
	raw := buf.Bytes()
	if len(raw) == 0 {
		t.Fatal("日志缓冲区为空")
	}
	var entry map[string]interface{}
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		return entry
	}
	t.Fatalf("未找到有效日志行: %s", string(raw))
	return nil
}

// TestLogger_Method 测试日志记录请求方法。
func TestLogger_Method(t *testing.T) {
	var buf bytes.Buffer
	r, restore := setupLoggerTest(&buf)
	defer restore()

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	logEntry := parseLog(t, &buf)
	method, ok := logEntry["msg"].(string)
	if !ok {
		t.Fatalf("日志缺少 msg 字段: %v", logEntry)
	}
	if method != "GET /test" {
		t.Errorf("期望 msg=GET /test，实际 %s", method)
	}
}

// TestLogger_StatusCode 测试日志记录状态码和日志级别。
func TestLogger_StatusCode(t *testing.T) {
	var buf bytes.Buffer
	r, restore := setupLoggerTest(&buf)
	defer restore()

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	logEntry := parseLog(t, &buf)
	status, ok := logEntry["status"].(float64)
	if !ok {
		t.Fatalf("日志缺少 status 字段: %v", logEntry)
	}
	if status != 200 {
		t.Errorf("期望 status=200，实际 %v", status)
	}
	// 200 应使用 INFO 级别
	level, _ := logEntry["level"].(string)
	if level != "INFO" {
		t.Errorf("期望 level=INFO，实际 %s", level)
	}
}

// TestLogger_ClientIP 测试日志记录客户端 IP。
func TestLogger_ClientIP(t *testing.T) {
	var buf bytes.Buffer
	r, restore := setupLoggerTest(&buf)
	defer restore()

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	logEntry := parseLog(t, &buf)
	if _, ok := logEntry["client_ip"]; !ok {
		t.Fatalf("日志缺少 client_ip 字段: %v", logEntry)
	}
}

// TestLogger_RequestID 测试日志包含 request_id。
func TestLogger_RequestID(t *testing.T) {
	var buf bytes.Buffer
	r, restore := setupLoggerTest(&buf)
	defer restore()

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "trace-001")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	logEntry := parseLog(t, &buf)
	rid, ok := logEntry["request_id"].(string)
	if !ok {
		t.Fatalf("日志缺少 request_id 字段: %v", logEntry)
	}
	if rid != "trace-001" {
		t.Errorf("期望 request_id=trace-001，实际 %s", rid)
	}
}

// TestLogger_WarnLevel 测试 4xx 状态码以 WARN 级别输出。
func TestLogger_WarnLevel(t *testing.T) {
	var buf bytes.Buffer
	r, restore := setupLoggerTest(&buf)
	defer restore()

	r.GET("/notfound", func(c *gin.Context) {
		c.String(404, "not found")
	})

	req := httptest.NewRequest("GET", "/notfound", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	logEntry := parseLog(t, &buf)
	level, _ := logEntry["level"].(string)
	if level != "WARN" {
		t.Errorf("4xx 应使用 WARN 级别，实际 %s", level)
	}
}

// TestLogger_ErrorLevel 测试 5xx 状态码以 ERROR 级别输出。
func TestLogger_ErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	r, restore := setupLoggerTest(&buf)
	defer restore()

	r.GET("/error", func(c *gin.Context) {
		c.String(500, "error")
	})

	req := httptest.NewRequest("GET", "/error", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	logEntry := parseLog(t, &buf)
	level, _ := logEntry["level"].(string)
	if level != "ERROR" {
		t.Errorf("5xx 应使用 ERROR 级别，实际 %s", level)
	}
}
