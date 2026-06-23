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

// safeHandler 安全获取 handler：ok 为 true 时调用 get() 返回真实 handler，否则返回 placeholder。
//
// get 仅在条件满足时调用，避免 nil deref panic。
// h 参数用于未来扩展（目前仅依赖 ok 判断）。
func safeHandler(h *Handlers, ok bool, get func() gin.HandlerFunc) gin.HandlerFunc {
	if h != nil && ok {
		return get()
	}
	return placeholder()
}
