// Package router 负责注册 Gin 路由。
//
// helpers.go 提供路由注册辅助函数，消除 portal.go / admin.go 中 ~150 行 if/else nil-check 样板。
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// placeholder 返回 501 占位处理器。
// safeHandler 在条件不满足时统一返回此函数。
func placeholder() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{
			"code":    501,
			"message": "功能未实现",
			"data":    nil,
		})
	}
}

// safeHandler 安全获取 handler：cond() 仅在 h != nil 时调用，避免 nil deref。
//
// cond 和 get 均为惰性求值——h 为 nil 时不会触发 panic。
func safeHandler(h *Handlers, cond func() bool, get func() gin.HandlerFunc) gin.HandlerFunc {
	if h != nil && cond() {
		return get()
	}
	return placeholder()
}
