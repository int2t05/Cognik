// Package router 负责注册 Gin 路由。
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// placeholder 返回 501 占位处理器。
func placeholder() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{
			"code":    501,
			"message": "功能未实现",
			"data":    nil,
		})
	}
}

// safeHandler 安全获取 handler，h 为 nil 时回退到 placeholder。
func safeHandler(h *Handlers, cond func() bool, get func() gin.HandlerFunc) gin.HandlerFunc {
	if h != nil && cond() {
		return get()
	}
	return placeholder()
}
