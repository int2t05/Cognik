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
// 权限字符串统一引用 permissions.go 常量，路由注册、RBAC 中间件、角色种子数据同一来源。
func registerAdminRoutes(rg *gin.RouterGroup, h *Handlers) {
	// 申告管理
	if h != nil && h.Ticket != nil {
		rg.GET("/tickets", middleware.RequirePermission(PermTicketRead), h.Ticket.ListAll)
		rg.GET("/tickets/:id", middleware.RequirePermission(PermTicketRead), h.Ticket.GetDetail)
		rg.PATCH("/tickets/:id/status", middleware.RequirePermission(PermTicketWrite), h.Ticket.UpdateStatus)
		rg.POST("/tickets/:id/records", middleware.RequirePermission(PermTicketWrite), h.Ticket.AddRecord)
	} else {
		rg.GET("/tickets", placeholder())
		rg.GET("/tickets/:id", placeholder())
		rg.PATCH("/tickets/:id/status", placeholder())
		rg.POST("/tickets/:id/records", placeholder())
	}
	// 知识库候选
	if h != nil && h.Ticket != nil {
		rg.POST("/tickets/:id/knowledge-candidate", middleware.RequirePermission(PermTicketWrite), h.Ticket.CreateKnowledgeCandidate)
	} else {
		rg.POST("/tickets/:id/knowledge-candidate", placeholder())
	}

	// 知识库管理
	if h != nil && h.Knowledge != nil {
		rg.GET("/knowledge-bases", middleware.RequirePermission(PermKnowledgeRead), h.Knowledge.ListKBs)
		rg.POST("/knowledge-bases", middleware.RequirePermission(PermKnowledgeWrite), h.Knowledge.CreateKB)
		rg.PUT("/knowledge-bases/:id", middleware.RequirePermission(PermKnowledgeWrite), h.Knowledge.UpdateKB)
		rg.DELETE("/knowledge-bases/:id", middleware.RequirePermission(PermKnowledgeWrite), h.Knowledge.DeleteKB)
		rg.GET("/knowledge-bases/:kb_id/articles", middleware.RequirePermission(PermKnowledgeRead), h.Knowledge.ListArticles)
		rg.POST("/knowledge-bases/:kb_id/articles", middleware.RequirePermission(PermKnowledgeWrite), h.Knowledge.CreateArticle)
		rg.PUT("/articles/:id", middleware.RequirePermission(PermKnowledgeWrite), h.Knowledge.UpdateArticle)
		rg.GET("/articles/:id", middleware.RequirePermission(PermKnowledgeRead), h.Knowledge.GetArticleDetail)
		rg.POST("/articles/:id/submit-review", middleware.RequirePermission(PermKnowledgeWrite), h.Knowledge.SubmitReview)
		rg.POST("/articles/:id/review", middleware.RequirePermission(PermKnowledgeReview), h.Knowledge.Review)
		rg.POST("/articles/:id/publish", middleware.RequirePermission(PermKnowledgeReview), h.Knowledge.Publish)
		rg.POST("/articles/:id/disable", middleware.RequirePermission(PermKnowledgeReview), h.Knowledge.Disable)
		rg.POST("/articles/:id/enable", middleware.RequirePermission(PermKnowledgeReview), h.Knowledge.Enable)
		rg.POST("/knowledge-bases/:kb_id/documents/upload", middleware.RequirePermission(PermKnowledgeWrite), h.Knowledge.UploadDocuments)
		rg.GET("/knowledge-bases/:kb_id/documents/:id/status", middleware.RequirePermission(PermKnowledgeRead), h.Knowledge.GetDocumentStatus)
		rg.POST("/knowledge-bases/:kb_id/documents/:id/retry", middleware.RequirePermission(PermKnowledgeWrite), h.Knowledge.RetryDocument)
	} else {
		rg.GET("/knowledge-bases", placeholder())
		rg.POST("/knowledge-bases", placeholder())
		rg.PUT("/knowledge-bases/:id", placeholder())
		rg.DELETE("/knowledge-bases/:id", placeholder())
		rg.GET("/knowledge-bases/:kb_id/articles", placeholder())
		rg.POST("/knowledge-bases/:kb_id/articles", placeholder())
		rg.PUT("/articles/:id", placeholder())
		rg.GET("/articles/:id", placeholder())
		rg.POST("/articles/:id/submit-review", placeholder())
		rg.POST("/articles/:id/review", placeholder())
		rg.POST("/articles/:id/publish", placeholder())
		rg.POST("/articles/:id/disable", placeholder())
		rg.POST("/articles/:id/enable", placeholder())
	}

	// 用户与角色管理
	userRoutes := rg.Group("/users")
	{
		if h != nil && h.User != nil {
			userRoutes.GET("", middleware.RequirePermission(PermUserManage), h.User.List)
			userRoutes.POST("", middleware.RequirePermission(PermUserManage), h.User.Create)
			userRoutes.GET("/:id", middleware.RequirePermission(PermUserManage), h.User.GetByID)
			userRoutes.PUT("/:id", middleware.RequirePermission(PermUserManage), h.User.Update)
			userRoutes.PATCH("/:id/freeze", middleware.RequirePermission(PermUserManage), h.User.Freeze)
			userRoutes.PATCH("/:id/unfreeze", middleware.RequirePermission(PermUserManage), h.User.Restore)
		} else {
			userRoutes.GET("", placeholder())
			userRoutes.POST("", placeholder())
			userRoutes.GET("/:id", placeholder())
			userRoutes.PUT("/:id", placeholder())
			userRoutes.PATCH("/:id/freeze", placeholder())
			userRoutes.PATCH("/:id/unfreeze", placeholder())
		}
	}

	roleRoutes := rg.Group("/roles")
	{
		if h != nil && h.Role != nil {
			roleRoutes.GET("", middleware.RequirePermission(PermUserManage), h.Role.List)
			roleRoutes.POST("", middleware.RequirePermission(PermUserManage), h.Role.Create)
			roleRoutes.GET("/:id", middleware.RequirePermission(PermUserManage), h.Role.GetByID)
			roleRoutes.PUT("/:id", middleware.RequirePermission(PermUserManage), h.Role.Update)
			roleRoutes.DELETE("/:id", middleware.RequirePermission(PermUserManage), h.Role.Delete)
		} else {
			roleRoutes.GET("", placeholder())
			roleRoutes.POST("", placeholder())
			roleRoutes.GET("/:id", placeholder())
			roleRoutes.PUT("/:id", placeholder())
			roleRoutes.DELETE("/:id", placeholder())
		}
	}

	// 菜单权限绑定
	if h != nil && h.Role != nil {
		rg.GET("/menus", middleware.RequirePermission(PermUserManage), h.Role.ListMenus)
		rg.PUT("/roles/:id/menus", middleware.RequirePermission(PermUserManage), h.Role.UpdateRoleMenus)
	} else {
		rg.GET("/menus", placeholder())
		rg.PUT("/roles/:id/menus", placeholder())
	}

	// 数据看板
	if h != nil && h.Dashboard != nil {
		rg.GET("/dashboard/stats", middleware.RequirePermission(PermDashboardRead), h.Dashboard.GetStats)
		rg.GET("/dashboard/trends", middleware.RequirePermission(PermDashboardRead), h.Dashboard.GetTrends)
	} else {
		rg.GET("/dashboard/stats", placeholder())
		rg.GET("/dashboard/trends", placeholder())
	}

	// 操作日志
	if h != nil && h.Audit != nil {
		rg.GET("/audit-logs", middleware.RequirePermission(PermAuditRead), h.Audit.List)
	} else {
		rg.GET("/audit-logs", placeholder())
	}

	// LLM 配置
	if h != nil && h.LLMConfig != nil {
		rg.GET("/llm-configs", middleware.RequirePermission(PermSystemConfig), h.LLMConfig.ListConfigs)
		rg.POST("/llm-configs", middleware.RequirePermission(PermSystemConfig), h.LLMConfig.CreateConfig)
		rg.GET("/llm-configs/:id", middleware.RequirePermission(PermSystemConfig), h.LLMConfig.GetConfig)
		rg.PUT("/llm-configs/:id", middleware.RequirePermission(PermSystemConfig), h.LLMConfig.UpdateConfig)
		rg.DELETE("/llm-configs/:id", middleware.RequirePermission(PermSystemConfig), h.LLMConfig.DeleteConfig)
		rg.POST("/llm-configs/:id/test", middleware.RequirePermission(PermSystemConfig), h.LLMConfig.TestConnection)
	} else {
		rg.GET("/llm-configs", placeholder())
		rg.POST("/llm-configs", placeholder())
		rg.GET("/llm-configs/:id", placeholder())
		rg.PUT("/llm-configs/:id", placeholder())
		rg.DELETE("/llm-configs/:id", placeholder())
		rg.POST("/llm-configs/:id/test", placeholder())
	}

	// 系统配置
	if h != nil && h.Config != nil {
		rg.GET("/configs/:key", middleware.RequirePermission(PermSystemConfig), h.Config.Get)
		rg.PUT("/configs/:key", middleware.RequirePermission(PermSystemConfig), h.Config.Update)
	} else {
		rg.GET("/configs/:key", placeholder())
		rg.PUT("/configs/:key", placeholder())
	}
}
