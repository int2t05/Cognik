// Package middleware 提供 Gin 中间件。
//
// 本文件实现请求日志中间件，通过 slog 输出结构化 JSON 日志到 stdout+文件。
// 日志字段：request_id、method、path、status、latency_ms、client_ip、user_id。
package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger 返回请求日志中间件。
//
// 每条请求结束时输出一条 slog 记录，自动携带 RequestID 中间件生成的 X-Request-ID。
// 状态码 ≥500 以 Error 级别输出，≥400 以 Warn 级别输出，其余 Info。
//
// Logger 不再接受 writer 参数——日志输出目标由 main.go 中 slog.SetDefault 统一控制。
// 测试中可通过 slog.SetDefault 设置测试 handler 验证日志行为。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		// 提取请求 ID（由 RequestID 中间件提前生成）
		requestID, _ := c.Get(RequestIDKey)

		// 提取已认证用户 ID（由 JWTAuth 中间件设置）
		var userID interface{}
		if uid, exists := c.Get("userID"); exists {
			userID = uid
		}

	attrs := []any{
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"status", status,
		"latency_ms", latency.Milliseconds(),
		"client_ip", c.ClientIP(),
	}
	if requestID != nil {
		attrs = append(attrs, "request_id", requestID)
	}
	if userID != nil {
		attrs = append(attrs, "user_id", userID)
	}
	// 业务错误码（由 response.Error 通过 c.Set 写入）
	if errCode, exists := c.Get("errCode"); exists {
		attrs = append(attrs, "err_code", errCode)
	}

	msg := c.Request.Method + " " + c.Request.URL.Path
		switch {
		case status >= 500:
			slog.Error(msg, attrs...)
		case status >= 400:
			slog.Warn(msg, attrs...)
		default:
			slog.Info(msg, attrs...)
		}
	}
}

// RequestIDFromContext 从 Gin context 提取请求 ID。
//
// 供 Service 层或其他中间件使用，将请求 ID 注入业务日志。
func RequestIDFromContext(c *gin.Context) string {
	if rid, exists := c.Get(RequestIDKey); exists {
		if s, ok := rid.(string); ok {
			return s
		}
	}
	return ""
}

// SlogWithRequest 从 Gin context 创建带 request_id 的 slog.Logger。
//
// 调用方在请求处理 goroutine 中使用返回的 logger，
// 业务日志自动携带请求 ID，与 HTTP 请求日志形成完整链路。
func SlogWithRequest(c *gin.Context) *slog.Logger {
	attrs := []any{"request_id", RequestIDFromContext(c)}
	if uid, exists := c.Get("userID"); exists {
		attrs = append(attrs, "user_id", uid)
	}
	return slog.With(attrs...)
}
