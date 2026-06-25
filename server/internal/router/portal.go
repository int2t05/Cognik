// Package router 负责注册 Gin 路由。
//
// 本文件注册门户端路由，所有路由需要 JWT 认证。
package router

import "github.com/gin-gonic/gin"

// registerPortalRoutes 注册门户端路由。
//
// 门户端面向报障人用户，提供智能问答、申告提交、进度查询等功能。
// 仅需 JWT 认证，不做角色校验——任何已登录用户均可使用门户功能。
func registerPortalRoutes(rg *gin.RouterGroup, h *Handlers) {
	// 知识库列表（门户端 Chat 需要选择知识库，无需 admin 权限）
	rg.GET("/knowledge-bases", safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.ListKBsForPortal }))

	// 智能问答 — 会话 CRUD + 流式对话
	rg.POST("/chat-sessions", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.CreateChatSession }))
	rg.GET("/chat-sessions", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.ListSessions }))
	rg.GET("/chat-sessions/:id", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.GetChatDetail }))
	rg.DELETE("/chat-sessions/:id", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.DeleteSession }))
	rg.PATCH("/chat-sessions/:id", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.UpdateSessionMeta }))
	rg.POST("/chat-sessions/:id/stream", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.StreamChatMessage }))
	rg.GET("/chat-sessions/:id/stream", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.ResumeStream }))
	rg.POST("/chat-sessions/:id/cancel", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.CancelGeneration }))
	rg.POST("/chat-sessions/:id/feedback", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.SubmitFeedback }))
	rg.POST("/chat-sessions/:id/messages/:msgId/feedback", safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.SubmitMessageFeedback }))

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
