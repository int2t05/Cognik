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

	"github.com/gin-gonic/gin"

	"opsmind/internal/cache"
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
// dbPing 用于 /readyz 健康检查探测 DB 连通性，nil 时跳过。
func Setup(cfg *config.AppConfig, userCache *cache.UserStatusCache, h *Handlers, dbPing func() error) *gin.Engine {
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

	// /health — 存活探针（K8s liveness），仅检查进程存活
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// /readyz — 就绪探针（K8s readiness），检查 DB 连通性
	r.GET("/readyz", func(c *gin.Context) {
		if dbPing != nil {
			if err := dbPing(); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": err.Error()})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// 公开系统配置（无需认证），直接注册在根引擎上避免 group 路由冲突
	if h != nil && h.Config != nil {
		r.GET("/api/v1/public/configs/:key", h.Config.GetPublic)
	} else {
		r.GET("/api/v1/public/configs/:key", placeholder())
	}

	public := r.Group("/api/v1/auth")
	registerPublicRoutes(public, h)

	authRequired := r.Group("/api/v1/auth/me")
	authRequired.Use(middleware.JWTAuth(userCache, cfg.JWT.Secret))
	registerAuthRequiredRoutes(authRequired, h)

	portal := r.Group("/api/v1/portal")
	portal.Use(middleware.JWTAuth(userCache, cfg.JWT.Secret))
	registerPortalRoutes(portal, h)

	admin := r.Group("/api/v1/admin")
	admin.Use(middleware.JWTAuth(userCache, cfg.JWT.Secret))
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
	if h.Message == nil {
		panic("opsmind: MessageHandler 未初始化")
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
