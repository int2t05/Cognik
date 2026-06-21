//go:build integration

// Package service_test 验证 Scheduler 后台调度器业务逻辑。
//
// 测试覆盖 PLAN.md Task30 定义的核心逻辑：
// TicketAutoCloseJob — 自动关闭超过 7 天的申告
package service_test

import (
	"fmt"
	"testing"
	"time"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"gorm.io/gorm"
)

var schedDB *gorm.DB

func init() {
	cfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "opsmind", Password: "opsmind_dev",
		DBName: "opsmind_test", SSLMode: "disable",
	}
	db, err := database.Init(cfg)
	if err != nil {
		panic(err)
	}
	schedDB = db
}

func setupSchedulerTest(t *testing.T) (*service.Scheduler, *model.User) {
	t.Helper()

	schedDB.Exec(`CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY, username VARCHAR(64) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL, real_name VARCHAR(64) NOT NULL,
		phone VARCHAR(11) NOT NULL, email VARCHAR(128),
		status SMALLINT NOT NULL DEFAULT 1, first_login BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	schedDB.Exec(`CREATE TABLE IF NOT EXISTS tickets (
		id BIGSERIAL PRIMARY KEY, ticket_no VARCHAR(32) NOT NULL UNIQUE,
		user_id BIGINT NOT NULL, title VARCHAR(255) NOT NULL, description TEXT NOT NULL,
		urgency SMALLINT NOT NULL, impact_scope SMALLINT DEFAULT 1,
		affected_systems JSONB, contact_phone VARCHAR(11) NOT NULL, contact_email VARCHAR(128),
		status SMALLINT NOT NULL DEFAULT 1, supplement_count SMALLINT NOT NULL DEFAULT 0,
		chat_context JSONB, source SMALLINT NOT NULL DEFAULT 1,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	// 清理旧数据
	schedDB.Exec("DELETE FROM ticket_records")
		schedDB.Exec("DELETE FROM tickets")
	schedDB.Exec("DELETE FROM users WHERE username LIKE 'sched_%'")

	// 创建测试用户
	now := time.Now()
	user := &model.User{
		Username:     fmt.Sprintf("sched_user_%d", now.UnixNano()),
		PasswordHash: "$2a$10$hash",
		RealName:     "测试用户",
		Phone:        fmt.Sprintf("1%010d", now.UnixNano()%10000000000),
		Status:       1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := schedDB.Create(user).Error; err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}

	ticketRepo := repository.NewTicketRepo(schedDB)
	ticketSvc := service.NewTicketService(ticketRepo, service.NewGormTxManager(schedDB), nil, nil)
	scheduler := service.NewScheduler(ticketSvc)

	return scheduler, user
}

// =============================================================================
// TicketAutoCloseJob
// =============================================================================

func TestScheduler_RunAutoClose_ClosesOldTickets(t *testing.T) {
	scheduler, user := setupSchedulerTest(t)

	// 创建 8 天前的申告（应被关闭）
	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	oldTicket := &model.Ticket{
		TicketNo: fmt.Sprintf("TK-OLD-%d", time.Now().UnixNano()),
		UserID: user.ID, Title: "旧申告", Description: "旧描述",
		Urgency: 1, ContactPhone: "13800000001", Status: 1, Source: 1,
		CreatedAt: oldTime, UpdatedAt: oldTime,
	}
	if err := schedDB.Create(oldTicket).Error; err != nil {
		t.Fatalf("创建旧申告失败: %v", err)
	}

	// 执行自动关闭
	closed, err := scheduler.RunAutoClose(time.Now().Add(-7 * 24 * time.Hour))
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if closed != 1 {
		t.Errorf("期望关闭 1 条, got %d", closed)
	}

	// 验证旧申告已关闭
	var updated model.Ticket
	schedDB.First(&updated, oldTicket.ID)
	if updated.Status != 5 {
		t.Errorf("期望旧申告 Status=5(已关闭), got %d", updated.Status)
	}
}

func TestScheduler_RunAutoClose_SkipsRecentTickets(t *testing.T) {
	scheduler, user := setupSchedulerTest(t)

	// 创建 6 天前的申告（不应被关闭，因为不足 7 天）
	recentTime := time.Now().Add(-6 * 24 * time.Hour)
	recent := &model.Ticket{
		TicketNo: fmt.Sprintf("TK-REC-%d", time.Now().UnixNano()),
		UserID: user.ID, Title: "新申告", Description: "新描述",
		Urgency: 1, ContactPhone: "13800000001", Status: 1, Source: 1,
		CreatedAt: recentTime, UpdatedAt: recentTime,
	}
	if err := schedDB.Create(recent).Error; err != nil {
		t.Fatalf("创建新申告失败: %v", err)
	}

	closed, err := scheduler.RunAutoClose(time.Now().Add(-7 * 24 * time.Hour))
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if closed != 0 {
		t.Errorf("期望关闭 0 条, got %d", closed)
	}

	// 验证新申告未被关闭
	var updated model.Ticket
	schedDB.First(&updated, recent.ID)
	if updated.Status != 1 {
		t.Errorf("期望新申告 Status=1, got %d", updated.Status)
	}
}

func TestScheduler_RunAutoClose_SkipsResolvedStatus(t *testing.T) {
	scheduler, user := setupSchedulerTest(t)

	oldTime := time.Now().Add(-8 * 24 * time.Hour)

	// 创建已解决状态的旧申告（不应被关闭）
	resolved := &model.Ticket{
		TicketNo: fmt.Sprintf("TK-RES-%d", time.Now().UnixNano()),
		UserID: user.ID, Title: "已解决", Description: "已解决",
		Urgency: 1, ContactPhone: "13800000001", Status: 4, Source: 1,
		CreatedAt: oldTime, UpdatedAt: oldTime,
	}
	if err := schedDB.Create(resolved).Error; err != nil {
		t.Fatalf("创建已解决申告失败: %v", err)
	}

	closed, err := scheduler.RunAutoClose(time.Now().Add(-7 * 24 * time.Hour))
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if closed != 0 {
		t.Errorf("期望关闭 0 条（已解决不受影响）, got %d", closed)
	}

	var updated model.Ticket
	schedDB.First(&updated, resolved.ID)
	if updated.Status != 4 {
		t.Errorf("期望已解决 Status=4, got %d", updated.Status)
	}
}

func TestScheduler_RunAutoClose_OnlyPendingProcessingSupplement(t *testing.T) {
	scheduler, user := setupSchedulerTest(t)

	oldTime := time.Now().Add(-8 * 24 * time.Hour)

	// 创建 3 种应关闭状态的旧申告
	for _, status := range []int16{1, 2, 3} {
		ticket := &model.Ticket{
			TicketNo: fmt.Sprintf("TK-S%d-%d", status, time.Now().UnixNano()),
			UserID: user.ID, Title: "待关闭", Description: "描述",
			Urgency: 1, ContactPhone: "13800000001", Status: status, Source: 1,
			CreatedAt: oldTime, UpdatedAt: oldTime,
		}
		if err := schedDB.Create(ticket).Error; err != nil {
			t.Fatalf("创建申告失败: %v", err)
		}
	}

	closed, err := scheduler.RunAutoClose(time.Now().Add(-7 * 24 * time.Hour))
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if closed != 3 {
		t.Errorf("期望关闭 3 条（status 1,2,3）, got %d", closed)
	}
}
