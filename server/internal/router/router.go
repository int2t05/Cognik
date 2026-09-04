// Package router 负责 Gin 路由注册（auth/portal/admin 三组）。
package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	llmconfig "opsmind/internal/domain/chat/llm_config"
	"opsmind/internal/domain/chat/session"
	"opsmind/internal/domain/knowledge"
	"opsmind/internal/domain/system/audit"
	sysconfig "opsmind/internal/domain/system/config"
	"opsmind/internal/domain/system/dashboard"
	"opsmind/internal/domain/system/message"
	"opsmind/internal/domain/ticket"
	"opsmind/internal/domain/user/account"
	"opsmind/internal/domain/user/auth"
	"opsmind/internal/domain/user/role"
	"opsmind/internal/infra/cache"
	"opsmind/internal/infra/config"
	"opsmind/internal/infra/middleware"
)

// Handlers 聚合所有 Handler 实例，供路由注册使用。
type Handlers struct {
	Auth      *auth.AuthHandler
	User      *account.UserHandler
	Role      *role.RoleHandler
	Knowledge *knowledge.KnowledgeHandler
	Ticket    *ticket.TicketHandler
	Chat      *session.ChatHandler
	Message   *message.MessageHandler
	Dashboard *dashboard.DashboardHandler
	Audit     *audit.AuditHandler
	Config    *sysconfig.ConfigHandler
	LLMConfig *llmconfig.LLMConfigHandler
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

	// 公开系统配置（无需认证）
	if h != nil && h.Config != nil {
		r.GET("/api/v1/public/configs/:key", h.Config.GetPublic)
		// 文章图片公开端点（img 标签无法携带 Authorization header）
		if h != nil && h.Knowledge != nil {
			r.GET("/api/v1/public/articles/:articleId/images/:filename", h.Knowledge.ServeArticleImage)
		} else {
			r.GET("/api/v1/public/articles/:articleId/images/:filename", placeholder())
		}
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

	// 上传配置（仅需 JWT 认证，供前端前置校验文件类型与大小）
	if h != nil && h.Knowledge != nil {
		r.GET("/api/v1/config/upload", middleware.JWTAuth(userCache, cfg.JWT.Secret), h.Knowledge.GetUploadConfig)
	} else {
		r.GET("/api/v1/config/upload", middleware.JWTAuth(userCache, cfg.JWT.Secret), placeholder())
	}

	return r
}

// assertHandlers 生产模式下验证所有 Handler 非 nil。
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
