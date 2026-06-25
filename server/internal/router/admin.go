// Package router 负责注册 Gin 路由。
//
// 本文件注册后台管理路由，所有路由需要 JWT 认证 + RBAC 权限（在 router.go 中统一挂载）。
package router

import (
	"opsmind/internal/middleware"

	"github.com/gin-gonic/gin"
)

// registerAdminRoutes 注册后台管理路由。
//
// 使用 safeHandler 消除 if/else nil-check 样板：
// 条件不满足时自动回退到 placeholder()，release 模式由 assertHandlers 保证不触发。
func registerAdminRoutes(rg *gin.RouterGroup, h *Handlers) {
	// 申告管理
	rg.GET("/tickets", middleware.RequirePermission(PermTicketRead), safeHandler(h, func() bool { return h.Ticket != nil }, func() gin.HandlerFunc { return h.Ticket.ListAll }))
	rg.GET("/tickets/:id", middleware.RequirePermission(PermTicketRead), safeHandler(h, func() bool { return h.Ticket != nil }, func() gin.HandlerFunc { return h.Ticket.GetDetailAdmin }))
	rg.PATCH("/tickets/:id/status", middleware.RequirePermission(PermTicketWrite), safeHandler(h, func() bool { return h.Ticket != nil }, func() gin.HandlerFunc { return h.Ticket.UpdateStatus }))
	rg.POST("/tickets/:id/records", middleware.RequirePermission(PermTicketWrite), safeHandler(h, func() bool { return h.Ticket != nil }, func() gin.HandlerFunc { return h.Ticket.AddRecord }))
	rg.POST("/tickets/:id/knowledge-candidate", middleware.RequirePermission(PermTicketWrite), safeHandler(h, func() bool { return h.Ticket != nil }, func() gin.HandlerFunc { return h.Ticket.CreateKnowledgeCandidate }))
	rg.POST("/tickets/batch-delete", middleware.RequirePermission(PermTicketWrite), safeHandler(h, func() bool { return h.Ticket != nil }, func() gin.HandlerFunc { return h.Ticket.BatchDelete }))

	// 知识库管理
	rg.GET("/knowledge-bases", middleware.RequirePermission(PermKnowledgeRead), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.ListKBs }))
	rg.POST("/knowledge-bases", middleware.RequirePermission(PermKnowledgeWrite), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.CreateKB }))
	rg.PUT("/knowledge-bases/:id", middleware.RequirePermission(PermKnowledgeWrite), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.UpdateKB }))
	rg.DELETE("/knowledge-bases/:id", middleware.RequirePermission(PermKnowledgeWrite), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.DeleteKB }))
	rg.GET("/knowledge-bases/:kb_id/articles", middleware.RequirePermission(PermKnowledgeRead), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.ListArticles }))
	rg.POST("/knowledge-bases/:kb_id/articles", middleware.RequirePermission(PermKnowledgeWrite), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.CreateArticle }))
	rg.PUT("/articles/:id", middleware.RequirePermission(PermKnowledgeWrite), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.UpdateArticle }))
	rg.GET("/articles/:id", middleware.RequirePermission(PermKnowledgeRead), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.GetArticleDetail }))
	rg.POST("/articles/:id/submit-review", middleware.RequirePermission(PermKnowledgeWrite), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.SubmitReview }))
	rg.POST("/articles/:id/review", middleware.RequirePermission(PermKnowledgeReview), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.Review }))
	rg.POST("/articles/:id/publish", middleware.RequirePermission(PermKnowledgeReview), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.Publish }))
	rg.POST("/articles/:id/disable", middleware.RequirePermission(PermKnowledgeReview), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.Disable }))
	rg.POST("/articles/:id/enable", middleware.RequirePermission(PermKnowledgeReview), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.Enable }))
	rg.DELETE("/articles/:id", middleware.RequirePermission(PermKnowledgeWrite), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.DeleteArticle }))
	rg.POST("/knowledge-bases/:kb_id/documents/upload", middleware.RequirePermission(PermKnowledgeWrite), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.UploadDocuments }))
	rg.GET("/knowledge-bases/:kb_id/documents/:id/status", middleware.RequirePermission(PermKnowledgeRead), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.GetDocumentStatus }))
	rg.POST("/knowledge-bases/:kb_id/documents/:id/retry", middleware.RequirePermission(PermKnowledgeWrite), safeHandler(h, func() bool { return h.Knowledge != nil }, func() gin.HandlerFunc { return h.Knowledge.RetryDocument }))

	// 用户管理
	userRoutes := rg.Group("/users")
	userRoutes.GET("", middleware.RequirePermission(PermUserManage), safeHandler(h, func() bool { return h.User != nil }, func() gin.HandlerFunc { return h.User.List }))
	userRoutes.POST("", middleware.RequirePermission(PermUserManage), safeHandler(h, func() bool { return h.User != nil }, func() gin.HandlerFunc { return h.User.Create }))
	userRoutes.GET("/:id", middleware.RequirePermission(PermUserManage), safeHandler(h, func() bool { return h.User != nil }, func() gin.HandlerFunc { return h.User.GetByID }))
	userRoutes.POST("/batch-delete", middleware.RequirePermission(PermUserManage), safeHandler(h, func() bool { return h.User != nil }, func() gin.HandlerFunc { return h.User.BatchDelete }))
	userRoutes.PUT("/:id", middleware.RequirePermission(PermUserManage), safeHandler(h, func() bool { return h.User != nil }, func() gin.HandlerFunc { return h.User.Update }))
	userRoutes.PATCH("/:id/freeze", middleware.RequirePermission(PermUserManage), safeHandler(h, func() bool { return h.User != nil }, func() gin.HandlerFunc { return h.User.Freeze }))
	userRoutes.PATCH("/:id/unfreeze", middleware.RequirePermission(PermUserManage), safeHandler(h, func() bool { return h.User != nil }, func() gin.HandlerFunc { return h.User.Restore }))

	// 角色管理
	roleRoutes := rg.Group("/roles")
	roleRoutes.GET("", middleware.RequirePermission(PermUserManage), safeHandler(h, func() bool { return h.Role != nil }, func() gin.HandlerFunc { return h.Role.List }))
	roleRoutes.POST("", middleware.RequirePermission(PermUserManage), safeHandler(h, func() bool { return h.Role != nil }, func() gin.HandlerFunc { return h.Role.Create }))
	roleRoutes.GET("/:id", middleware.RequirePermission(PermUserManage), safeHandler(h, func() bool { return h.Role != nil }, func() gin.HandlerFunc { return h.Role.GetByID }))
	roleRoutes.PUT("/:id", middleware.RequirePermission(PermUserManage), safeHandler(h, func() bool { return h.Role != nil }, func() gin.HandlerFunc { return h.Role.Update }))
	roleRoutes.DELETE("/:id", middleware.RequirePermission(PermUserManage), safeHandler(h, func() bool { return h.Role != nil }, func() gin.HandlerFunc { return h.Role.Delete }))

	// 菜单权限绑定
	rg.GET("/menus", middleware.RequirePermission(PermUserManage), safeHandler(h, func() bool { return h.Role != nil }, func() gin.HandlerFunc { return h.Role.ListMenus }))
	rg.PUT("/roles/:id/menus", middleware.RequirePermission(PermUserManage), safeHandler(h, func() bool { return h.Role != nil }, func() gin.HandlerFunc { return h.Role.UpdateRoleMenus }))

	// 数据看板
	rg.GET("/dashboard/stats", middleware.RequirePermission(PermDashboardRead), safeHandler(h, func() bool { return h.Dashboard != nil }, func() gin.HandlerFunc { return h.Dashboard.GetStats }))
	rg.GET("/dashboard/trends", middleware.RequirePermission(PermDashboardRead), safeHandler(h, func() bool { return h.Dashboard != nil }, func() gin.HandlerFunc { return h.Dashboard.GetTrends }))
	rg.POST("/feedback/analyze", middleware.RequirePermission(PermDashboardRead), safeHandler(h, func() bool { return h.Chat != nil }, func() gin.HandlerFunc { return h.Chat.AnalyzeFeedback }))

	// 操作日志
	rg.GET("/audit-logs", middleware.RequirePermission(PermAuditRead), safeHandler(h, func() bool { return h.Audit != nil }, func() gin.HandlerFunc { return h.Audit.List }))
	rg.POST("/audit-logs/batch-delete", middleware.RequirePermission(PermAuditRead), safeHandler(h, func() bool { return h.Audit != nil }, func() gin.HandlerFunc { return h.Audit.BatchDelete }))

	// LLM 配置
	rg.GET("/llm-configs", middleware.RequirePermission(PermSystemConfig), safeHandler(h, func() bool { return h.LLMConfig != nil }, func() gin.HandlerFunc { return h.LLMConfig.ListConfigs }))
	rg.POST("/llm-configs", middleware.RequirePermission(PermSystemConfig), safeHandler(h, func() bool { return h.LLMConfig != nil }, func() gin.HandlerFunc { return h.LLMConfig.CreateConfig }))
	rg.GET("/llm-configs/:id", middleware.RequirePermission(PermSystemConfig), safeHandler(h, func() bool { return h.LLMConfig != nil }, func() gin.HandlerFunc { return h.LLMConfig.GetConfig }))
	rg.PUT("/llm-configs/:id", middleware.RequirePermission(PermSystemConfig), safeHandler(h, func() bool { return h.LLMConfig != nil }, func() gin.HandlerFunc { return h.LLMConfig.UpdateConfig }))
	rg.DELETE("/llm-configs/:id", middleware.RequirePermission(PermSystemConfig), safeHandler(h, func() bool { return h.LLMConfig != nil }, func() gin.HandlerFunc { return h.LLMConfig.DeleteConfig }))
	rg.POST("/llm-configs/:id/test", middleware.RequirePermission(PermSystemConfig), safeHandler(h, func() bool { return h.LLMConfig != nil }, func() gin.HandlerFunc { return h.LLMConfig.TestConnection }))

	// 系统配置
	rg.GET("/configs/:key", middleware.RequirePermission(PermSystemConfig), safeHandler(h, func() bool { return h.Config != nil }, func() gin.HandlerFunc { return h.Config.Get }))
	rg.PUT("/configs/:key", middleware.RequirePermission(PermSystemConfig), safeHandler(h, func() bool { return h.Config != nil }, func() gin.HandlerFunc { return h.Config.Update }))

	// 置信度阈值计算
	rg.POST("/confidence/compute-thresholds", middleware.RequirePermission(PermSystemConfig), safeHandler(h, func() bool { return h.Config != nil }, func() gin.HandlerFunc { return h.Config.ComputeThresholds }))
}
