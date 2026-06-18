// Package router 负责注册 Gin 路由。
//
// 路由分为三组：
// - /api/v1/auth — 公开路由（登录、刷新令牌等）
// - /api/v1/portal — 门户端路由（需要 JWT 认证）
// - /api/v1/admin — 后台管理路由（需要 JWT 认证 + RBAC 权限）
package router

import (
	"net/http"
	"strings"

	"gorm.io/gorm"

	"github.com/gin-gonic/gin"

	"opsmind/internal/config"
	"opsmind/internal/handler"
	"opsmind/internal/middleware"
)

// Handlers 聚合所有 Handler 实例，供路由注册使用。
type Handlers struct {
	Auth      *handler.AuthHandler
	User      *handler.UserHandler
	Role      *handler.RoleHandler
	Knowledge *handler.KnowledgeHandler
	Ticket    *handler.TicketHandler
	Chat      *handler.ChatHandler
	Message   *handler.MessageHandler
	Dashboard *handler.DashboardHandler
	Audit     *handler.AuditHandler
	Config    *handler.ConfigHandler
	LLMConfig *handler.LLMConfigHandler
}

// Setup 初始化 Gin 引擎并注册所有路由。
func Setup(cfg *config.AppConfig, db *gorm.DB, h *Handlers) *gin.Engine {
	gin.SetMode(cfg.Server.Mode)

	// 生产模式下，nil Handler 应立即失败而非返回运行时 501
	if cfg.Server.Mode == "release" {
		assertHandlers(h)
	}

	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.CORS(parseCORSOrigins(cfg.CORS.AllowOrigins), cfg.Server.Mode))
	r.Use(middleware.Logger())

	// /health — 存活探针（K8s liveness）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// /readyz — 就绪探针（K8s readiness），验证数据库可达
	r.GET("/readyz", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	public := r.Group("/api/v1/auth")
	registerPublicRoutes(public, h)

	authRequired := r.Group("/api/v1/auth/me")
	authRequired.Use(middleware.JWTAuth(db, cfg.JWT.Secret))
	registerAuthRequiredRoutes(authRequired, h)

	portal := r.Group("/api/v1/portal")
	portal.Use(middleware.JWTAuth(db, cfg.JWT.Secret))
	registerPortalRoutes(portal, h)

	admin := r.Group("/api/v1/admin")
	admin.Use(middleware.JWTAuth(db, cfg.JWT.Secret))
	registerAdminRoutes(admin, h)

	return r
}

// assertHandlers 生产模式下验证所有 Handler 非 nil，防止未装配模块以 501 暴露给用户。
func assertHandlers(h *Handlers) {
	if h == nil {
		panic("opsmind: Handlers 为 nil，装配错误")
	}
	if h.Auth == nil {
		panic("opsmind: AuthHandler 未初始化")
	}
	if h.User == nil {
		panic("opsmind: UserHandler 未初始化")
	}
	if h.Role == nil {
		panic("opsmind: RoleHandler 未初始化")
	}
	if h.Knowledge == nil {
		panic("opsmind: KnowledgeHandler 未初始化")
	}
	if h.Ticket == nil {
		panic("opsmind: TicketHandler 未初始化")
	}
	if h.Chat == nil {
		panic("opsmind: ChatHandler 未初始化")
	}
	if h.Dashboard == nil {
		panic("opsmind: DashboardHandler 未初始化")
	}
	if h.Audit == nil {
		panic("opsmind: AuditHandler 未初始化")
	}
	if h.Config == nil {
		panic("opsmind: ConfigHandler 未初始化")
	}
	if h.LLMConfig == nil {
		panic("opsmind: LLMConfigHandler 未初始化")
	}
}

func registerPublicRoutes(rg *gin.RouterGroup, h *Handlers) {
	if h != nil && h.Auth != nil {
		rg.POST("/login", h.Auth.Login)
		rg.POST("/refresh", h.Auth.Refresh)
	} else {
		rg.POST("/login", placeholder())
		rg.POST("/refresh", placeholder())
	}
}

func registerAuthRequiredRoutes(rg *gin.RouterGroup, h *Handlers) {
	if h != nil && h.Auth != nil {
		rg.POST("/change-password", h.Auth.ChangePassword)
		rg.POST("/logout", h.Auth.Logout)
	} else {
		rg.POST("/change-password", placeholder())
		rg.POST("/logout", placeholder())
	}
}

func parseCORSOrigins(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
