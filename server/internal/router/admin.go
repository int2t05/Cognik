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
// 后台管理面向运维人员和管理员，提供申告处理、知识库管理、用户管理等功能。
// 已实现 Handler 的路由绑定真实 Handler，未实现的仍使用占位。
func registerAdminRoutes(rg *gin.RouterGroup, h *Handlers) {
	// TODO(router/admin): 权限字符串散落在路由注册中，建议集中成常量或权限表。
	// 这样前端菜单、种子数据、RBAC 中间件能共享同一来源，减少拼写漂移。
	// 申告管理（T24 — 已实现）
	if h != nil && h.Ticket != nil {
		rg.GET("/tickets", middleware.RequirePermission("ticket:read"), h.Ticket.ListAll)
		rg.GET("/tickets/:id", middleware.RequirePermission("ticket:read"), h.Ticket.GetDetail)
		rg.PATCH("/tickets/:id/status", middleware.RequirePermission("ticket:write"), h.Ticket.UpdateStatus)
		rg.POST("/tickets/:id/records", middleware.RequirePermission("ticket:write"), h.Ticket.AddRecord)
	} else {
		rg.GET("/tickets", placeholder())
		rg.GET("/tickets/:id", placeholder())
		rg.PATCH("/tickets/:id/status", placeholder())
		rg.POST("/tickets/:id/records", placeholder())
	}
	// 知识库候选（从申告生成知识条目）
	if h != nil && h.Ticket != nil {
		rg.POST("/tickets/:id/knowledge-candidate", middleware.RequirePermission("ticket:write"), h.Ticket.CreateKnowledgeCandidate)
	} else {
		rg.POST("/tickets/:id/knowledge-candidate", placeholder())
	}

	// 知识库管理（T18 — 已实现）
	if h != nil && h.Knowledge != nil {
		rg.GET("/knowledge-bases", middleware.RequirePermission("knowledge:read"), h.Knowledge.ListKBs)
		rg.POST("/knowledge-bases", middleware.RequirePermission("knowledge:write"), h.Knowledge.CreateKB)
		rg.PUT("/knowledge-bases/:id", middleware.RequirePermission("knowledge:write"), h.Knowledge.UpdateKB)
		rg.DELETE("/knowledge-bases/:id", middleware.RequirePermission("knowledge:write"), h.Knowledge.DeleteKB)
		rg.GET("/knowledge-bases/:kb_id/articles", middleware.RequirePermission("knowledge:read"), h.Knowledge.ListArticles)
		rg.POST("/knowledge-bases/:kb_id/articles", middleware.RequirePermission("knowledge:write"), h.Knowledge.CreateArticle)
		rg.PUT("/articles/:id", middleware.RequirePermission("knowledge:write"), h.Knowledge.UpdateArticle)
		rg.GET("/articles/:id", middleware.RequirePermission("knowledge:read"), h.Knowledge.GetArticleDetail)
		rg.POST("/articles/:id/submit-review", middleware.RequirePermission("knowledge:write"), h.Knowledge.SubmitReview)
		rg.POST("/articles/:id/review", middleware.RequirePermission("knowledge:review"), h.Knowledge.Review)
		rg.POST("/articles/:id/publish", middleware.RequirePermission("knowledge:review"), h.Knowledge.Publish)
		rg.POST("/articles/:id/disable", middleware.RequirePermission("knowledge:review"), h.Knowledge.Disable)
		rg.POST("/articles/:id/enable", middleware.RequirePermission("knowledge:review"), h.Knowledge.Enable)
		// 文档上传/状态/重试
		rg.POST("/knowledge-bases/:kb_id/documents/upload", middleware.RequirePermission("knowledge:write"), h.Knowledge.UploadDocuments)
		rg.GET("/knowledge-bases/:kb_id/documents/:id/status", middleware.RequirePermission("knowledge:read"), h.Knowledge.GetDocumentStatus)
		rg.POST("/knowledge-bases/:kb_id/documents/:id/retry", middleware.RequirePermission("knowledge:write"), h.Knowledge.RetryDocument)
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

	// 用户管理（T14 — 已实现）
	userRoutes := rg.Group("/users")
	{
		if h != nil && h.User != nil {
			userRoutes.GET("", middleware.RequirePermission("user:manage"), h.User.List)
			userRoutes.POST("", middleware.RequirePermission("user:manage"), h.User.Create)
			userRoutes.GET("/:id", middleware.RequirePermission("user:manage"), h.User.GetByID)
			userRoutes.PUT("/:id", middleware.RequirePermission("user:manage"), h.User.Update)
			userRoutes.PATCH("/:id/freeze", middleware.RequirePermission("user:manage"), h.User.Freeze)
			userRoutes.PATCH("/:id/unfreeze", middleware.RequirePermission("user:manage"), h.User.Restore)
		} else {
			userRoutes.GET("", placeholder())
			userRoutes.POST("", placeholder())
			userRoutes.GET("/:id", placeholder())
			userRoutes.PUT("/:id", placeholder())
			userRoutes.PATCH("/:id/freeze", placeholder())
			userRoutes.PATCH("/:id/unfreeze", placeholder())
		}
	}

	// 角色权限（T15 — 已实现）
	roleRoutes := rg.Group("/roles")
	{
		if h != nil && h.Role != nil {
			roleRoutes.GET("", middleware.RequirePermission("user:manage"), h.Role.List)
			roleRoutes.POST("", middleware.RequirePermission("user:manage"), h.Role.Create)
			roleRoutes.GET("/:id", middleware.RequirePermission("user:manage"), h.Role.GetByID)
			roleRoutes.PUT("/:id", middleware.RequirePermission("user:manage"), h.Role.Update)
			roleRoutes.DELETE("/:id", middleware.RequirePermission("user:manage"), h.Role.Delete)
		} else {
			roleRoutes.GET("", placeholder())
			roleRoutes.POST("", placeholder())
			roleRoutes.GET("/:id", placeholder())
			roleRoutes.PUT("/:id", placeholder())
			roleRoutes.DELETE("/:id", placeholder())
		}
	}

	// 菜单（T15 菜单权限绑定）
	if h != nil && h.Role != nil {
		rg.GET("/menus", middleware.RequirePermission("user:manage"), h.Role.ListMenus)
		rg.PUT("/roles/:id/menus", middleware.RequirePermission("user:manage"), h.Role.UpdateRoleMenus)
	} else {
		rg.GET("/menus", placeholder())
		rg.PUT("/roles/:id/menus", placeholder())
	}

	// 数据看板（T32 — 已实现）
	// TODO(router/admin): dashboard 使用 audit:read 权限不够直观。
	// 建议引入 dashboard:read，避免拥有审计权限就天然拥有运营看板权限。
	if h != nil && h.Dashboard != nil {
		rg.GET("/dashboard/stats", middleware.RequirePermission("dashboard:read"), h.Dashboard.GetStats)
		rg.GET("/dashboard/trends", middleware.RequirePermission("dashboard:read"), h.Dashboard.GetTrends)
	} else {
		rg.GET("/dashboard/stats", placeholder())
		rg.GET("/dashboard/trends", placeholder())
	}

	// 操作日志（T33 — 已实现）
	if h != nil && h.Audit != nil {
		rg.GET("/audit-logs", middleware.RequirePermission("audit:read"), h.Audit.List)
	} else {
		rg.GET("/audit-logs", placeholder())
	}

	// LLM 配置
	if h != nil && h.LLMConfig != nil {
		rg.GET("/llm-configs", middleware.RequirePermission("system:config"), h.LLMConfig.ListConfigs)
		rg.POST("/llm-configs", middleware.RequirePermission("system:config"), h.LLMConfig.CreateConfig)
		rg.GET("/llm-configs/:id", middleware.RequirePermission("system:config"), h.LLMConfig.GetConfig)
		rg.PUT("/llm-configs/:id", middleware.RequirePermission("system:config"), h.LLMConfig.UpdateConfig)
		rg.DELETE("/llm-configs/:id", middleware.RequirePermission("system:config"), h.LLMConfig.DeleteConfig)
		rg.POST("/llm-configs/:id/test", middleware.RequirePermission("system:config"), h.LLMConfig.TestConnection)
	} else {
		rg.GET("/llm-configs", placeholder())
		rg.POST("/llm-configs", placeholder())
		rg.GET("/llm-configs/:id", placeholder())
		rg.PUT("/llm-configs/:id", placeholder())
		rg.DELETE("/llm-configs/:id", placeholder())
		rg.POST("/llm-configs/:id/test", placeholder())
	}

	// 系统配置（T34 — 已实现）
	if h != nil && h.Config != nil {
		rg.GET("/configs/:key", middleware.RequirePermission("system:config"), h.Config.Get)
		rg.PUT("/configs/:key", middleware.RequirePermission("system:config"), h.Config.Update)
	} else {
		rg.GET("/configs/:key", placeholder())
		rg.PUT("/configs/:key", placeholder())
	}
}
