// Package router 负责注册 Gin 路由。
package router

import "github.com/gin-gonic/gin"

// registerPortalRoutes 注册门户端路由（仅需 JWT 认证）。
func registerPortalRoutes(rg *gin.RouterGroup, h *Handlers) {
	// 知识库列表（门户端 Chat 需要选择知识库，无需 admin 权限）
	rg.GET("/knowledge-bases", safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.ListKBsForPortal }))

	// Agent 对话 — 线程 CRUD + 流式对话（SQLite 存储，与业务库隔离）
	rg.POST("/threads", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.CreateThread }))
	rg.GET("/threads", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.ListThreads }))
	rg.GET("/threads/:id", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.GetThreadDetail }))
	rg.DELETE("/threads/:id", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.DeleteThread }))
	rg.PATCH("/threads/:id", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.UpdateThread }))
	rg.POST("/threads/:id/stream", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.StreamChat }))
	rg.GET("/threads/:id/stream", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.ResumeStream }))
	rg.POST("/threads/:id/cancel", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.CancelGeneration }))

	// 申告管理
	rg.POST("/tickets", safeHandler(h, func() bool { return h.Ticket != nil }, func() gin.HandlerFunc { return h.Ticket.CreateTicket }))
	rg.GET("/tickets", safeHandler(h, func() bool { return h.Ticket != nil }, func() gin.HandlerFunc { return h.Ticket.ListByUser }))
	rg.GET("/tickets/:id", safeHandler(h, func() bool { return h.Ticket != nil }, func() gin.HandlerFunc { return h.Ticket.GetDetailPortal }))
	rg.PATCH("/tickets/:id/supplement", safeHandler(h, func() bool { return h.Ticket != nil }, func() gin.HandlerFunc { return h.Ticket.SupplementTicket }))
	rg.PATCH("/tickets/:id", safeHandler(h, func() bool { return h.Ticket != nil }, func() gin.HandlerFunc { return h.Ticket.UpdateTicket }))

	// 站内消息
	rg.GET("/messages", safeHandler(h, func() bool { return h.Message != nil }, func() gin.HandlerFunc { return h.Message.ListMessages }))
	rg.PUT("/messages/read-all", safeHandler(h, func() bool { return h.Message != nil }, func() gin.HandlerFunc { return h.Message.MarkAllRead }))
	rg.PUT("/messages/:id/read", safeHandler(h, func() bool { return h.Message != nil }, func() gin.HandlerFunc { return h.Message.MarkAsRead }))
	rg.GET("/messages/unread-count", safeHandler(h, func() bool { return h.Message != nil }, func() gin.HandlerFunc { return h.Message.CountUnread }))
}
