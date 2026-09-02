// Package middleware 提供 Gin 中间件。
package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger 返回请求日志中间件，输出结构化 slog 记录。
// 状态码 ≥500 Error，≥400 Warn，其余 Info。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		// 提取请求 ID（由 RequestID 中间件生成）
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
		// 业务错误码（response.Error 写入）
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
