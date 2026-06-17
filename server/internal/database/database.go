// Package database 负责初始化 PostgreSQL 数据库连接。
//
// 使用 GORM 作为 ORM 框架，通过 gorm.io/driver/postgres 连接 PostgreSQL。
// RAG 向量检索由 pgvector 扩展承担，通过 adapter/pgvector_store.go 访问。
package database

import (
	"fmt"
	"net/url"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"opsmind/internal/config"
)

// Init 初始化数据库连接。
//
// DSN 使用 URL 格式（postgres://）避免密码中特殊字符导致的连接失败。
// 连接池参数由配置控制，默认值适配单实例 MVP 部署。
func Init(cfg config.DatabaseConfig) (*gorm.DB, error) {
	// 使用 URL 格式构造 DSN，自动处理密码中空格、单引号等特殊字符
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		url.PathEscape(cfg.User),
		url.QueryEscape(cfg.Password),
		cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode,
	)

	// 生产环境降低日志级别，避免 SQL 泄露到标准输出
	logLevel := logger.Info
	if cfg.SSLMode == "require" || cfg.SSLMode == "verify-full" {
		logLevel = logger.Warn
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 sql.DB 失败: %w", err)
	}
	maxOpen := cfg.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 25
	}
	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 10
	}
	connLifetime := cfg.ConnMaxLifetime
	if connLifetime <= 0 {
		connLifetime = 5 * time.Minute
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(connLifetime)

	// 启动时验证连接可达性
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库 Ping 失败: %w", err)
	}

	return db, nil
}
