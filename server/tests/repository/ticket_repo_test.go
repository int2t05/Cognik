//go:build integration

// Package repository_test 验证 TicketRepo 数据访问层。
//
// 测试覆盖 PLAN.md Task23 定义的 10 个方法：
// Ticket: Create/FindByID/Update/UpdateStatus/IncrementSupplementCount/ListByUser/ListAll/AutoCloseTickets
// TicketRecord: CreateRecord/FindByTicketID
package repository_test

import (
	"fmt"
	"testing"
	"time"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
	"opsmind/internal/repository"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTicketTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbCfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "opsmind", Password: "opsmind_dev",
		DBName: "opsmind_test", SSLMode: "disable",
	}
	db, err := database.Init(dbCfg)
	if err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY, username VARCHAR(64) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL, real_name VARCHAR(64) NOT NULL,
		phone VARCHAR(11) NOT NULL, email VARCHAR(128),
		status SMALLINT NOT NULL DEFAULT 1, first_login BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS tickets (
		id BIGSERIAL PRIMARY KEY, ticket_no VARCHAR(32) NOT NULL UNIQUE,
		user_id BIGINT NOT NULL, title VARCHAR(255) NOT NULL, description TEXT NOT NULL,
		urgency SMALLINT NOT NULL, impact_scope SMALLINT DEFAULT 1,
		affected_systems JSONB, contact_phone VARCHAR(11) NOT NULL, contact_email VARCHAR(128),
		status SMALLINT NOT NULL DEFAULT 1, supplement_count SMALLINT NOT NULL DEFAULT 0,
		chat_context JSONB, source SMALLINT NOT NULL DEFAULT 1,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS ticket_records (
		id BIGSERIAL PRIMARY KEY, ticket_id BIGINT NOT NULL, operator_id BIGINT NOT NULL,
		action VARCHAR(32) NOT NULL, content TEXT, detail JSONB,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	return db
}

func cleanTicketTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	// 按 FK 依赖逆序清理，避免外键约束冲突
	db.Exec("DELETE FROM ticket_records")
	db.Exec("DELETE FROM tickets")
	db.Exec("DELETE FROM chat_messages")          // FK → chat_sessions
	db.Exec("DELETE FROM chat_sessions")          // FK → users/knowledge_bases
	db.Exec("DELETE FROM knowledge_chunks")       // FK → knowledge_articles
	db.Exec("DELETE FROM knowledge_articles")     // FK → knowledge_bases
	db.Exec("DELETE FROM knowledge_bases")        // FK → users
	db.Exec("DELETE FROM users WHERE username LIKE 'test_%'")
}

// createTestUser 创建测试用户。
func createTestUser(t *testing.T, db *gorm.DB, username string) *model.User {
	t.Helper()
	now := time.Now()
	u := &model.User{Username: username, PasswordHash: "$2a$10$hash", RealName: "测试", Phone: "13800000001", Status: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}
	return u
}

// =============================================================================
// Ticket 测试
// =============================================================================

func TestTicketRepo_Create(t *testing.T) {
	db := setupTicketTestDB(t)
	cleanTicketTables(t, db)
	repo := repository.NewTicketRepo(db)
	user := createTestUser(t, db, "test_ticket_user")

	ticket := &model.Ticket{
		TicketNo:     "TK-20260609-0001",
		UserID:       user.ID,
		Title:        "网络连接异常",
		Description:  "办公区网络频繁断开",
		Urgency:      2,
		ImpactScope:  1,
		ContactPhone: "13800000001",
		ContactEmail: "test@example.com",
		Status:       1,
		Source:       1,
	}

	err := repo.Create(ticket)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if ticket.ID == 0 {
		t.Error("Create 后应自动填充 ID")
	}
}

func TestTicketRepo_FindByID(t *testing.T) {
	db := setupTicketTestDB(t)
	cleanTicketTables(t, db)
	repo := repository.NewTicketRepo(db)
	user := createTestUser(t, db, "test_find_ticket")

	ticket := &model.Ticket{
		TicketNo: "TK-20260609-0002", UserID: user.ID, Title: "测试申告",
		Description: "描述", Urgency: 1, ContactPhone: "13800000001", Status: 1, Source: 1,
	}
	requireNoErr(t, db.Create(ticket).Error)

	// 添加处理记录
	record := &model.TicketRecord{TicketID: ticket.ID, OperatorID: user.ID, Action: "create", Content: "创建申告"}
	requireNoErr(t, db.Create(record).Error)

	got, err := repo.FindByID(ticket.ID)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if got.Title != "测试申告" {
		t.Errorf("期望 Title='测试申告', got '%s'", got.Title)
	}
	// 预加载 User
	if got.User.Username != "test_find_ticket" {
		t.Errorf("期望预加载 User, got '%s'", got.User.Username)
	}
	// 预加载 TicketRecords
	if len(got.TicketRecords) != 1 {
		t.Errorf("期望 1 条记录, got %d", len(got.TicketRecords))
	}
}

func TestTicketRepo_FindByID_NotFound(t *testing.T) {
	db := setupTicketTestDB(t)
	repo := repository.NewTicketRepo(db)

	got, err := repo.FindByID(999999)
	if err == nil {
		t.Fatal("期望错误, got nil")
	}
	if err != gorm.ErrRecordNotFound {
		t.Errorf("期望 gorm.ErrRecordNotFound, got %v", err)
	}
	if got != nil {
		t.Error("期望 nil")
	}
}

func TestTicketRepo_Update(t *testing.T) {
	db := setupTicketTestDB(t)
	cleanTicketTables(t, db)
	repo := repository.NewTicketRepo(db)
	user := createTestUser(t, db, "test_update_ticket")

	ticket := &model.Ticket{
		TicketNo: "TK-20260609-0003", UserID: user.ID, Title: "旧标题",
		Description: "旧描述", Urgency: 1, ContactPhone: "13800000001", Status: 1, Source: 1,
	}
	requireNoErr(t, db.Create(ticket).Error)

	ticket.Title = "新标题"
	ticket.Description = "新描述"
	err := repo.Update(ticket)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	var updated model.Ticket
	db.First(&updated, ticket.ID)
	if updated.Title != "新标题" {
		t.Errorf("期望 Title='新标题', got '%s'", updated.Title)
	}
}

func TestTicketRepo_UpdateStatus(t *testing.T) {
	db := setupTicketTestDB(t)
	cleanTicketTables(t, db)
	repo := repository.NewTicketRepo(db)
	user := createTestUser(t, db, "test_status_ticket")

	ticket := &model.Ticket{
		TicketNo: "TK-20260609-0004", UserID: user.ID, Title: "状态测试",
		Description: "描述", Urgency: 1, ContactPhone: "13800000001", Status: 1, Source: 1,
	}
	requireNoErr(t, db.Create(ticket).Error)

	rows, err := repo.UpdateStatus(ticket.ID, int(ticket.Status), 2) // 待处理 → 处理中 (CAS)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if rows != 1 {
		t.Errorf("期望 RowsAffected=1, got %d", rows)
	}

	var updated model.Ticket
	db.First(&updated, ticket.ID)
	if updated.Status != 2 {
		t.Errorf("期望 Status=2, got %d", updated.Status)
	}
}

func TestTicketRepo_IncrementSupplementCount(t *testing.T) {
	db := setupTicketTestDB(t)
	cleanTicketTables(t, db)
	repo := repository.NewTicketRepo(db)
	user := createTestUser(t, db, "test_supp_ticket")

	ticket := &model.Ticket{
		TicketNo: "TK-20260609-0005", UserID: user.ID, Title: "补充计数",
		Description: "描述", Urgency: 1, ContactPhone: "13800000001", Status: 2, Source: 1,
		SupplementCount: 1,
	}
	requireNoErr(t, db.Create(ticket).Error)

	ok, err := repo.IncrementSupplementCount(ticket.ID)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if !ok {
		t.Fatal("supplement_count=1 时应成功自增")
	}

	var updated model.Ticket
	db.First(&updated, ticket.ID)
	if updated.SupplementCount != 2 {
		t.Errorf("期望 SupplementCount=2, got %d", updated.SupplementCount)
	}
}

// TestTicketRepo_IncrementSupplementCount_Exceeded 验证超限时原子拒绝。
//
// 修复前：IncrementSupplementCount 无条件自增 supplement_count + 1，
// 并发请求可绕过 Service 层检查将计数推到 4、5。
// 修复后：使用 WHERE supplement_count < 3 的原子 UPDATE，
// ok=false 表示因超限未执行自增。
func TestTicketRepo_IncrementSupplementCount_Exceeded(t *testing.T) {
	db := setupTicketTestDB(t)
	cleanTicketTables(t, db)
	repo := repository.NewTicketRepo(db)
	user := createTestUser(t, db, "test_supp_exceeded")

	ticket := &model.Ticket{
		TicketNo: "TK-20260609-SUPP3", UserID: user.ID, Title: "超限测试",
		Description: "描述", Urgency: 1, ContactPhone: "x", Status: 2, Source: 1,
		SupplementCount: 3,
	}
	requireNoErr(t, db.Create(ticket).Error)

	ok, err := repo.IncrementSupplementCount(ticket.ID)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if ok {
		t.Fatal("supplement_count=3 时 IncrementSupplementCount 应返回 ok=false（超限拒绝）")
	}

	// 验证计数未被错误自增
	var updated model.Ticket
	db.First(&updated, ticket.ID)
	if updated.SupplementCount != 3 {
		t.Errorf("supplement_count 应保持 3, 实际 %d", updated.SupplementCount)
	}
}

func TestTicketRepo_ListByUser(t *testing.T) {
	db := setupTicketTestDB(t)
	cleanTicketTables(t, db)
	repo := repository.NewTicketRepo(db)
	user := createTestUser(t, db, "test_listbyuser")

	for i := 0; i < 3; i++ {
		ticket := &model.Ticket{
			TicketNo: fmt.Sprintf("TK-20260609-001%d", i), UserID: user.ID,
			Title: "申告", Description: "描述", Urgency: 1, ContactPhone: "13800000001", Status: 1, Source: 1,
		}
		requireNoErr(t, db.Create(ticket).Error)
	}

	tickets, total, err := repo.ListByUser(user.ID, 1, 10)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if total != 3 {
		t.Errorf("期望 total=3, got %d", total)
	}
	if len(tickets) != 3 {
		t.Errorf("期望 3 条, got %d", len(tickets))
	}
}

func TestTicketRepo_ListAll(t *testing.T) {
	db := setupTicketTestDB(t)
	cleanTicketTables(t, db)
	repo := repository.NewTicketRepo(db)
	user := createTestUser(t, db, "test_listall")

	// 创建不同状态和紧急程度的申告
	tickets := []model.Ticket{
		{TicketNo: "TK-20260609-0101", UserID: user.ID, Title: "待处理", Description: "d", Urgency: 2, ContactPhone: "x", Status: 1, Source: 1},
		{TicketNo: "TK-20260609-0102", UserID: user.ID, Title: "处理中", Description: "d", Urgency: 1, ContactPhone: "x", Status: 2, Source: 1},
		{TicketNo: "TK-20260609-0103", UserID: user.ID, Title: "高紧急", Description: "d", Urgency: 3, ContactPhone: "x", Status: 1, Source: 1},
	}
	for i := range tickets {
		requireNoErr(t, db.Create(&tickets[i]).Error)
	}

	// 过滤 status=1
	result, total, err := repo.ListAll(1, 0, 1, 10)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if total != 2 {
		t.Errorf("期望 total=2 (status=1), got %d", total)
	}

	// 全查询
	result, total, err = repo.ListAll(-1, 0, 1, 10)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if total != 3 {
		t.Errorf("期望 total=3 (all), got %d", total)
	}
	_ = result
}

func TestTicketRepo_AutoCloseTickets(t *testing.T) {
	db := setupTicketTestDB(t)
	cleanTicketTables(t, db)
	repo := repository.NewTicketRepo(db)
	user := createTestUser(t, db, "test_autoclose")

	// 创建 8 天前的申告
	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	old := &model.Ticket{
		TicketNo: "TK-20260601-0001", UserID: user.ID, Title: "旧申告",
		Description: "描述", Urgency: 1, ContactPhone: "x", Status: 1, Source: 1,
		CreatedAt: oldTime, UpdatedAt: oldTime,
	}
	requireNoErr(t, db.Create(&old).Error)

	// 创建今天的申告
	recent := &model.Ticket{
		TicketNo: "TK-20260609-0001", UserID: user.ID, Title: "新申告",
		Description: "描述", Urgency: 1, ContactPhone: "x", Status: 1, Source: 1,
	}
	requireNoErr(t, db.Create(&recent).Error)

	// 关闭 7 天前的申告
	ids, err := repo.AutoCloseTickets(time.Now().Add(-7 * 24 * time.Hour))
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("期望关闭 1 条, got %d", len(ids))
	}

	// 验证旧申告已关闭
	var oldUpdated model.Ticket
	db.First(&oldUpdated, old.ID)
	if oldUpdated.Status != 5 {
		t.Errorf("期望旧申告 Status=5(已关闭), got %d", oldUpdated.Status)
	}

	// 验证新申告未关闭
	var recentUpdated model.Ticket
	db.First(&recentUpdated, recent.ID)
	if recentUpdated.Status != 1 {
		t.Errorf("期望新申告 Status=1, got %d", recentUpdated.Status)
	}
}

// =============================================================================
// TicketRecord 测试
// =============================================================================

func TestTicketRepo_CreateRecord(t *testing.T) {
	db := setupTicketTestDB(t)
	cleanTicketTables(t, db)
	repo := repository.NewTicketRepo(db)
	user := createTestUser(t, db, "test_record")

	// 先创建申告（FK 约束：ticket_records.ticket_id → tickets.id）
	ticket := &model.Ticket{
		TicketNo: "TK-TEST-001", UserID: user.ID, Title: "测试", Description: "测试",
		Urgency: 1, ContactPhone: "13800000001", Status: 1, Source: 1,
	}
	require.NoError(t, db.Create(ticket).Error)

	record := &model.TicketRecord{
		TicketID:   ticket.ID,
		OperatorID: user.ID,
		Action:     "start",
		Content:    "开始处理申告",
	}

	err := repo.CreateRecord(record)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if record.ID == 0 {
		t.Error("CreateRecord 后应自动填充 ID")
	}
}

func TestTicketRepo_FindByTicketID(t *testing.T) {
	db := setupTicketTestDB(t)
	cleanTicketTables(t, db)
	repo := repository.NewTicketRepo(db)

	// 先创建用户和申告（FK 约束）
	user := createTestUser(t, db, "test_find_records")
	ticket := &model.Ticket{
		TicketNo: "TK-FIND-RECORDS", UserID: user.ID, Title: "记录查询测试",
		Description: "测试", Urgency: 1, ContactPhone: "13800000001", Status: 1, Source: 1,
	}
	requireNoErr(t, db.Create(ticket).Error)

	records := []model.TicketRecord{
		{TicketID: ticket.ID, OperatorID: user.ID, Action: "create", Content: "创建"},
		{TicketID: ticket.ID, OperatorID: user.ID, Action: "start", Content: "开始处理"},
	}
	for i := range records {
		requireNoErr(t, db.Create(&records[i]).Error)
	}

	got, err := repo.FindByTicketID(ticket.ID)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if len(got) != 2 {
		t.Errorf("期望 2 条记录, got %d", len(got))
	}
	// 验证按时间正序
	if got[0].Action != "create" {
		t.Errorf("期望第一条 action='create', got '%s'", got[0].Action)
	}
}

// requireNoErr 简化错误断言。
func requireNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
}
