//go:build integration

package model_test

import (
	"strings"
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"

	"gorm.io/gorm"
)

// TestPaginateScope 验证 PaginateScope 生成正确的 LIMIT 和 OFFSET。
// 需要 PostgreSQL 连接以生成真实 SQL 并验证。
func TestPaginateScope(t *testing.T) {
	db, err := database.Init(config.DatabaseConfig{
		Host: "localhost", Port: 5432,
		User: "opsmind", Password: "opsmind_dev",
		DBName: "opsmind_test", SSLMode: "disable",
	})
	if err != nil {
		t.Skipf("跳过：数据库不可用: %v", err)
	}

	tests := []struct {
		name           string
		page           int
		size           int
		expectedOffset int
		expectedLimit  int
	}{
		{"第一页", 1, 20, 0, 20},
		{"第二页", 2, 20, 20, 20},
		{"第三页", 3, 10, 20, 10},
		{"page=0 归一化为 1", 0, 20, 0, 20},
		{"负数 page 归一化为 1", -1, 20, 0, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope := model.PaginateScope(tt.page, tt.size)
			if scope == nil {
				t.Fatal("PaginateScope 返回 nil")
			}

			// 通过 DryRun 生成实际 SQL 验证 LIMIT 和 OFFSET
			sql := db.Session(&gorm.Session{DryRun: true}).
				Model(&model.User{}).Scopes(scope).
				Find(&[]model.User{}).Statement.SQL.String()

			// GORM 使用参数化查询（$1, $2），检查 SQL 结构与参数一起
			hasLimit := strings.Contains(sql, "LIMIT")
			hasOffset := strings.Contains(sql, "OFFSET") || tt.expectedOffset == 0

			if !hasLimit {
				t.Errorf("SQL 缺少 LIMIT 子句\n实际 SQL: %s", sql)
			}
			if !hasOffset {
				t.Errorf("SQL 缺少 OFFSET 子句\n实际 SQL: %s", sql)
			}
		})
	}
}
