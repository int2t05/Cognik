// Package database 负责初始化 PostgreSQL 连接（GORM + pgvector）。
package database

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"cognik/internal/infra/config"
)

// Init 初始化数据库连接，DSN 使用 URL 格式以处理密码中特殊字符。
// 启动时连接失败支持指数退避重试（默认 5 次），避免 DB 未就绪即退出。
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

	maxAttempts := 5
	if s := os.Getenv("COGNIK_DB_CONNECT_MAX_ATTEMPTS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			maxAttempts = n
		}
	}

	var db *gorm.DB
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		db, err = openAndProbe(dsn, logLevel, cfg)
		if err == nil {
			if attempt > 1 {
				slog.Info("数据库重连成功", "attempts", attempt)
			}
			return db, nil
		}
		if attempt == maxAttempts {
			return nil, fmt.Errorf("数据库连接失败（尝试 %d 次）: %w", attempt, err)
		}
		backoff := time.Duration(1<<(attempt-1)) * time.Second // 1s,2s,4s,8s...
		slog.Warn("数据库连接失败，退避后重试", "attempt", attempt, "backoff", backoff, "error", err)
		time.Sleep(backoff)
	}
	return nil, fmt.Errorf("数据库连接失败")
}

// openAndProbe 打开 GORM 连接、配置连接池并 Ping 验证可达性。
func openAndProbe(dsn string, logLevel logger.LogLevel, cfg config.DatabaseConfig) (*gorm.DB, error) {
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
	// 0 = 不限制（PostgreSQL max_connections 兜底）
	connLifetime := cfg.ConnMaxLifetime
	if connLifetime <= 0 {
		connLifetime = 5 * time.Minute
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(connLifetime)

	// 启动时验证连接可达性
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库 Ping 失败: %w", err)
	}
	return db, nil
}
