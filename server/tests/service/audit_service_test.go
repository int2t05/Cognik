//go:build integration

// Package service_test 验证 AuditService 业务逻辑层。
package service_test

import (
	"context"
	"os"
	"strconv"
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func setupAuditServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	port, _ := strconv.Atoi(envOrDefault("TEST_DB_PORT", "5432"))
	db, err := database.Init(config.DatabaseConfig{
		Host:     envOrDefault("TEST_DB_HOST", "localhost"), Port: port,
		User:     envOrDefault("TEST_DB_USER", "opsmind"), Password: envOrDefault("TEST_DB_PASSWORD", "opsmind_dev"),
		DBName:   envOrDefault("TEST_DB_NAME", "opsmind_test"), SSLMode: envOrDefault("TEST_DB_SSLMODE", "disable"),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY, username VARCHAR(64) NOT NULL, password_hash VARCHAR(255) NOT NULL,
		real_name VARCHAR(64) NOT NULL, phone VARCHAR(20) NOT NULL,
		status SMALLINT NOT NULL DEFAULT 1, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS audit_logs (
		id BIGSERIAL PRIMARY KEY, operator_id BIGINT NOT NULL,
		action VARCHAR(64) NOT NULL, target_type VARCHAR(32),
		target_id BIGINT, detail JSONB, ip_address VARCHAR(45),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	// 注意：测试期望空表起始，必须清空所有审计日志而非仅 test_ 前缀
	db.Exec("DELETE FROM audit_logs")
	return db
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
	return def
}

func TestAuditService_List_All(t *testing.T) {
	db := setupAuditServiceTestDB(t)
	repo := repository.NewAuditRepo(db)
	svc := service.NewAuditService(repo)
	ctx := context.Background()

	db.Exec(`INSERT INTO users (id, username, password_hash, real_name, phone) VALUES (1, 'admin', '$2a$10$x', '测试用户', '13800000001') ON CONFLICT DO NOTHING`)
	repo.Create(ctx, &model.AuditLog{OperatorID: 1, Action: "test_login", TargetType: "user", TargetID: 1, Detail: datatypes.JSON(`{"ip":"127.0.0.1"}`), IPAddress: "127.0.0.1"})

	_, total, err := svc.List(ctx, repository.AuditFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if total < 1 {
		t.Errorf("期望 total>=1, 实际 %d", total)
	}
}

func TestAuditService_List_ByAction(t *testing.T) {
	db := setupAuditServiceTestDB(t)
	repo := repository.NewAuditRepo(db)
	svc := service.NewAuditService(repo)
	ctx := context.Background()

	repo.Create(ctx, &model.AuditLog{OperatorID: 1, Action: "test_action_a", TargetType: "user", TargetID: 1})
	repo.Create(ctx, &model.AuditLog{OperatorID: 1, Action: "test_action_b", TargetType: "ticket", TargetID: 1})

	_, total, err := svc.List(ctx, repository.AuditFilter{Action: "test_action_a", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if total != 1 {
		t.Errorf("action 过滤: 期望 total=1, 实际 %d", total)
	}
}

func TestAuditService_List_ByOperator(t *testing.T) {
	db := setupAuditServiceTestDB(t)
	repo := repository.NewAuditRepo(db)
	svc := service.NewAuditService(repo)
	ctx := context.Background()

	// 使用唯一 operator_id 避免与其他测试的用户数据冲突
	const testOpID = 99001
	db.Exec(`INSERT INTO users (id, username, password_hash, real_name, phone, created_at, updated_at)
		VALUES (?, 'audit_op_test', 'hashed_pwd_test', '操作人A', '13800000001', NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET real_name = '操作人A', updated_at = NOW()`,
		testOpID)
	repo.Create(ctx, &model.AuditLog{OperatorID: testOpID, Action: "test_op1", TargetType: "user", TargetID: 1})

	items, _, err := svc.List(ctx, repository.AuditFilter{OperatorID: testOpID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	for _, item := range items {
		if item.OperatorID == testOpID && item.OperatorName != "操作人A" {
			t.Errorf("期望操作人A, 实际 %s", item.OperatorName)
		}
	}
}

func TestAuditService_List_Pagination(t *testing.T) {
	db := setupAuditServiceTestDB(t)
	repo := repository.NewAuditRepo(db)
	svc := service.NewAuditService(repo)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		repo.Create(ctx, &model.AuditLog{OperatorID: 0, Action: "test_page", TargetType: "user", TargetID: 1})
	}

	items, total, err := svc.List(ctx, repository.AuditFilter{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if total != 5 {
		t.Errorf("期望 total=5, 实际 %d", total)
	}
	if len(items) != 2 {
		t.Errorf("第1页: 期望 2 条, 实际 %d", len(items))
	}
}

func TestAuditService_List_Empty(t *testing.T) {
	db := setupAuditServiceTestDB(t)
	repo := repository.NewAuditRepo(db)
	svc := service.NewAuditService(repo)
	ctx := context.Background()

	items, total, err := svc.List(ctx, repository.AuditFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List 空表: %v", err)
	}
	if total != 0 {
		t.Errorf("空表: 期望 total=0, 实际 %d", total)
	}
	if len(items) != 0 {
		t.Errorf("空表: 期望空列表, 实际 %d 条", len(items))
	}
}
