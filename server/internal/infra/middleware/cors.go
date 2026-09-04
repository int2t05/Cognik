// Package middleware 提供 Gin 中间件。
package middleware

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS 返回 CORS 跨域中间件。
// 非 release 模式动态回 Echo Origin，release 模式严格使用配置列表 + DNS 重绑定防护。
func CORS(allowOrigins []string, mode string) gin.HandlerFunc {
	isRelease := mode == "release"

	if isRelease {
		if len(allowOrigins) == 0 {
			slog.Error("生产模式必须配置 COGNOS_CORS_ALLOW_ORIGINS，拒绝以空值启动")
			panic("CORS: release 模式不允许空 AllowOrigins")
		}
		for _, origin := range allowOrigins {
			if origin == "*" {
				panic("CORS: AllowCredentials=true 时 AllowOrigins 不能包含 \"*\"")
			}
			if strings.Contains(origin, "localhost") {
				slog.Warn("CORS AllowOrigin 包含 localhost，生产环境存在 DNS 重绑定风险", "origin", origin)
			}
		}
	}

	cfg := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length", "X-Request-Id", "X-Request-ID", "X-Total-Count"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}

	if isRelease {
		cfg.AllowOrigins = allowOrigins
	} else {
		// 开发模式：允许任意 Origin
		cfg.AllowOriginFunc = func(origin string) bool {
			return true
		}
	}

	return cors.New(cfg)
}
