// Package database 负责初始化 PostgreSQL 连接（GORM + pgvector）。
package database

import (
	"fmt"
	"net/url"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"cognos/internal/infra/config"
)

// Init 初始化数据库连接，DSN 使用 URL 格式以处理密码中特殊字符。
func Init(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		url.PathEscape(cfg.User),
		url.QueryEscape(cfg.Password),
		cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode,
	)

	// SSL 模式为 require 时降低日志级别，避免 SQL 泄露
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

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 sql.DB 失败: %w", err)
	}
	maxOpen := cfg.MaxOpenConns
	maxIdle := cfg.MaxIdleConns
	// 0 = 不限制（PostgreSQL max_connections 兜底）
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
