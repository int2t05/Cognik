//go:build integration

// Package database_test 验证数据库连接和迁移功能。
//
// 需要运行中的 PostgreSQL 实例。使用 integration build tag 跳过：
//
//	go test ./tests/database/... -tags=integration
package database_test

import (
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
)

// testDBConfig 返回测试数据库配置
func testDBConfig() config.DatabaseConfig {
	return config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "opsmind",
		Password: "opsmind_dev",
		DBName:   "opsmind_test",
		SSLMode:  "disable",
	}
}

func TestInit_Connection(t *testing.T) {
	db, err := database.Init(testDBConfig())
	if err != nil {
		t.Fatalf("Init() 失败: %v", err)
	}

	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("Ping 失败: %v", err)
	}
}

func TestInit_ConnectionPool(t *testing.T) {
	db, err := database.Init(testDBConfig())
	if err != nil {
		t.Fatalf("Init() 失败: %v", err)
	}

	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	// 验证连接池配置
	stats := sqlDB.Stats()
	if stats.MaxOpenConnections != 0 {
		t.Errorf("MaxOpenConnections = %d, 期望 0（不限）", stats.MaxOpenConnections)
	}
}
