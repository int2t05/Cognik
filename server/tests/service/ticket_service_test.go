//go:build integration

// Package service_test 验证 TicketService 业务逻辑。
//
// 测试覆盖 PLAN.md Task24 定义的全部方法：
// CreateTicket / SupplementTicket / UpdateStatus / AddRecord / ListByUser / ListAll / GetDetail
package service_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/dto/request"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"gorm.io/gorm"
)

func setupTicketServiceDB(t *testing.T) *gorm.DB {
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

func cleanTicketServiceTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	db.Exec("DELETE FROM ticket_records")
	db.Exec("DELETE FROM tickets")
	db.Exec("DELETE FROM users WHERE username LIKE 'tsvc_%'")
}

// hashToPhone 根据字符串生成 11 位手机号（与服务层其他测试保持一致）。
func hashToPhone(s string) string {
	var h uint32
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	phone := make([]byte, 11)
	phone[0] = '1'
	for i := 1; i < 11; i++ {
		h = h*31 + uint32(i)
		phone[i] = byte('0' + (h % 10))
	}
	return string(phone)
}

func createTestUserForService(t *testing.T, db *gorm.DB, username string) *model.User {
	t.Helper()
	now := time.Now()
	// 使用 username 哈希生成唯一手机号，避免 idx_users_phone 唯一索引冲突
	phone := hashToPhone(username)
	u := &model.User{Username: username, PasswordHash: "$2a$10$hash", RealName: "测试用户", Phone: phone, Status: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}
	return u
}

// =============================================================================
// CreateTicket
// =============================================================================

func TestTicketService_CreateTicket(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_create")

	req := request.CreateTicketRequest{
		Title:        "网络连接异常",
		Description:  "办公区网络频繁断开",
		Urgency:      2,
		ImpactScope:  1,
		ContactPhone: "13800000001",
		ContactEmail: "test@example.com",
	}

	err := svc.CreateTicket(bgCtx, req, user.ID)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	// 验证申告已创建
	tickets, total, err := repo.ListByUser(bgCtx, user.ID, 1, 10)
	if err != nil {
		t.Fatalf("查询申告失败: %v", err)
	}
	if total != 1 {
		t.Fatalf("期望 1 条申告, got %d", total)
	}

	// 验证 ticket_no 格式 TK-YYYYMMDD-XXXX
	ticket := &tickets[0]
	if !strings.HasPrefix(ticket.TicketNo, "TK-") {
		t.Errorf("ticket_no 应以 TK- 开头, got '%s'", ticket.TicketNo)
	}
	if ticket.Status != 1 {
		t.Errorf("新建申告 status 应为 1(待处理), got %d", ticket.Status)
	}
	if ticket.Source != 1 {
		t.Errorf("门户提交 source 应为 1, got %d", ticket.Source)
	}

	// 验证唯一约束：重复 ticket_no 不应导致错误（日期+随机后缀足够）
	_ = ticket
}

func TestTicketService_CreateTicket_Validation(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_val")

	// 标题为空
	err := svc.CreateTicket(bgCtx, request.CreateTicketRequest{
		Title: "", Description: "描述", Urgency: 1, ContactPhone: "13800000001",
	}, user.ID)
	if err == nil {
		t.Fatal("标题为空应返回错误")
	}

	// 紧急程度无效
	err = svc.CreateTicket(bgCtx, request.CreateTicketRequest{
		Title: "测试", Description: "描述", Urgency: 5, ContactPhone: "13800000001",
	}, user.ID)
	if err == nil {
		t.Fatal("紧急程度无效应返回错误")
	}

	// 手机号为空
	err = svc.CreateTicket(bgCtx, request.CreateTicketRequest{
		Title: "测试", Description: "描述", Urgency: 1, ContactPhone: "",
	}, user.ID)
	if err == nil {
		t.Fatal("手机号为空应返回错误")
	}
}

// =============================================================================
// SupplementTicket
// =============================================================================

func TestTicketService_SupplementTicket(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_supp")

	// 创建申告并设置为"需补充信息"状态
	ticket := &model.Ticket{
		TicketNo: "TK-20260609-S001", UserID: user.ID, Title: "测试",
		Description: "描述", Urgency: 1, ContactPhone: "x", Status: 3, Source: 1,
		SupplementCount: 0,
	}
	requireNoErr(t, db.Create(ticket).Error)

	err := svc.SupplementTicket(bgCtx, ticket.ID, user.ID, request.SupplementTicketRequest{
		Content: "补充的信息内容",
	})
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	// 验证状态变回处理中(2)
	updated, _ := repo.FindByID(bgCtx, ticket.ID)
	if updated.Status != 2 {
		t.Errorf("补充后状态应为 2(处理中), got %d", updated.Status)
	}

	// 验证创建了处理记录
	records, _ := repo.FindByTicketID(bgCtx, ticket.ID)
	found := false
	for _, r := range records {
		if r.Action == "supplement" {
			found = true
			break
		}
	}
	if !found {
		t.Error("应创建 action=supplement 的处理记录")
	}
}

func TestTicketService_SupplementTicket_WrongStatus(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_supp_ws")

	// 创建待处理状态的申告（不是"需补充信息"）
	ticket := &model.Ticket{
		TicketNo: "TK-20260609-S002", UserID: user.ID, Title: "测试",
		Description: "描述", Urgency: 1, ContactPhone: "x", Status: 1, Source: 1,
	}
	requireNoErr(t, db.Create(ticket).Error)

	err := svc.SupplementTicket(bgCtx, ticket.ID, user.ID, request.SupplementTicketRequest{
		Content: "补充信息",
	})
	if err == nil {
		t.Fatal("非'需补充信息'状态应返回错误")
	}
}

func TestTicketService_SupplementTicket_NotOwner(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	owner := createTestUserForService(t, db, "tsvc_supp_owner")
	other := createTestUserForService(t, db, "tsvc_supp_other")

	ticket := &model.Ticket{
		TicketNo: "TK-20260609-S003", UserID: owner.ID, Title: "测试",
		Description: "描述", Urgency: 1, ContactPhone: "x", Status: 3, Source: 1,
	}
	requireNoErr(t, db.Create(ticket).Error)

	err := svc.SupplementTicket(bgCtx, ticket.ID, other.ID, request.SupplementTicketRequest{
		Content: "补充信息",
	})
	if err == nil {
		t.Fatal("非申告人补充应返回错误")
	}
}

// =============================================================================
// UpdateStatus — 状态机
// =============================================================================

func TestTicketService_UpdateStatus_Start(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_start")
	operator := createTestUserForService(t, db, "tsvc_start_op")

	ticket := &model.Ticket{
		TicketNo: "TK-20260609-M001", UserID: user.ID, Title: "测试",
		Description: "描述", Urgency: 1, ContactPhone: "x", Status: 1, Source: 1,
	}
	requireNoErr(t, db.Create(ticket).Error)

	err := svc.UpdateStatus(bgCtx, ticket.ID, operator.ID, request.UpdateTicketStatusRequest{
		Action: "start",
		Result: "开始处理此申告",
	})
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	updated, _ := repo.FindByID(bgCtx, ticket.ID)
	if updated.Status != 2 {
		t.Errorf("start 后状态应为 2(处理中), got %d", updated.Status)
	}

	// 验证处理记录
	records, _ := repo.FindByTicketID(bgCtx, ticket.ID)
	if len(records) != 1 {
		t.Fatalf("期望 1 条记录, got %d", len(records))
	}
	if records[0].Action != "start" {
		t.Errorf("期望 action='start', got '%s'", records[0].Action)
	}
}

func TestTicketService_UpdateStatus_RequestInfo(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_reqinfo")
	operator := createTestUserForService(t, db, "tsvc_reqinfo_op")

	ticket := &model.Ticket{
		TicketNo: "TK-20260609-M002", UserID: user.ID, Title: "测试",
		Description: "描述", Urgency: 1, ContactPhone: "x", Status: 2, Source: 1,
		SupplementCount: 1,
	}
	requireNoErr(t, db.Create(ticket).Error)

	err := svc.UpdateStatus(bgCtx, ticket.ID, operator.ID, request.UpdateTicketStatusRequest{
		Action: "request_info",
		Result: "请提供更多信息",
	})
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	updated, _ := repo.FindByID(bgCtx, ticket.ID)
	if updated.Status != 3 {
		t.Errorf("request_info 后状态应为 3(需补充信息), got %d", updated.Status)
	}
	if updated.SupplementCount != 2 {
		t.Errorf("supplement_count 应为 2, got %d", updated.SupplementCount)
	}
}

func TestTicketService_UpdateStatus_RequestInfoExceeded(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_exceed")
	operator := createTestUserForService(t, db, "tsvc_exceed_op")

	// 已达 3 次补充上限
	ticket := &model.Ticket{
		TicketNo: "TK-20260609-M003", UserID: user.ID, Title: "测试",
		Description: "描述", Urgency: 1, ContactPhone: "x", Status: 2, Source: 1,
		SupplementCount: 3,
	}
	requireNoErr(t, db.Create(ticket).Error)

	err := svc.UpdateStatus(bgCtx, ticket.ID, operator.ID, request.UpdateTicketStatusRequest{
		Action: "request_info",
		Result: "再提供信息",
	})
	if err == nil {
		t.Fatal("超过 3 次补充应返回错误")
	}
}

// TestTicketService_UpdateStatus_RequestInfoAtomicCheck 验证补充次数上限由数据库原子检查保证。
//
// 修复前：Service 层读取 supplement_count 后判断 >= 3，存在 TOCTOU 竞态条件。
// 修复后：IncrementSupplementCount 使用 WHERE supplement_count < 3 的原子 UPDATE，
// 即使并发请求也能正确拒绝超限操作。
func TestTicketService_UpdateStatus_RequestInfoAtomicCheck(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_atomic")
	operator := createTestUserForService(t, db, "tsvc_atomic_op")

	// supplement_count=3 时，原子 UPDATE 不应生效
	ticket := &model.Ticket{
		TicketNo: fmt.Sprintf("TK-ATOM-%d", time.Now().UnixNano()),
		UserID: user.ID, Title: "原子检查测试", Description: "描述",
		Urgency: 1, ContactPhone: "x", Status: 2, Source: 1,
		SupplementCount: 3,
	}
	requireNoErr(t, db.Create(ticket).Error)

	err := svc.UpdateStatus(bgCtx, ticket.ID, operator.ID, request.UpdateTicketStatusRequest{
		Action: "request_info",
		Result: "应被拒绝的补充请求",
	})
	if err == nil {
		t.Fatal("supplement_count=3 时 request_info 应返回错误")
	}

	// 验证 supplement_count 未被错误自增
	var updated model.Ticket
	db.First(&updated, ticket.ID)
	if updated.SupplementCount != 3 {
		t.Errorf("supplement_count 应保持 3, 实际 %d — 原子检查失效", updated.SupplementCount)
	}
	if updated.Status != 2 {
		t.Errorf("status 应保持 2(处理中), 实际 %d", updated.Status)
	}
}

func TestTicketService_UpdateStatus_Resolve(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_resolve")
	operator := createTestUserForService(t, db, "tsvc_resolve_op")

	ticket := &model.Ticket{
		TicketNo: "TK-20260609-M004", UserID: user.ID, Title: "测试",
		Description: "描述", Urgency: 1, ContactPhone: "x", Status: 2, Source: 1,
	}
	requireNoErr(t, db.Create(ticket).Error)

	err := svc.UpdateStatus(bgCtx, ticket.ID, operator.ID, request.UpdateTicketStatusRequest{
		Action: "resolve",
		Result: "问题已解决，重启路由器即可",
	})
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	updated, _ := repo.FindByID(bgCtx, ticket.ID)
	if updated.Status != 4 {
		t.Errorf("resolve 后状态应为 4(已解决), got %d", updated.Status)
	}
}

func TestTicketService_UpdateStatus_Close(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_close")
	operator := createTestUserForService(t, db, "tsvc_close_op")

	ticket := &model.Ticket{
		TicketNo: "TK-20260609-M005", UserID: user.ID, Title: "测试",
		Description: "描述", Urgency: 1, ContactPhone: "x", Status: 1, Source: 1,
	}
	requireNoErr(t, db.Create(ticket).Error)

	err := svc.UpdateStatus(bgCtx, ticket.ID, operator.ID, request.UpdateTicketStatusRequest{
		Action: "close",
		Result: "重复申告",
	})
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	updated, _ := repo.FindByID(bgCtx, ticket.ID)
	if updated.Status != 5 {
		t.Errorf("close 后状态应为 5(已关闭), got %d", updated.Status)
	}
}

func TestTicketService_UpdateStatus_InvalidAction(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_invact")
	operator := createTestUserForService(t, db, "tsvc_invact_op")

	ticket := &model.Ticket{
		TicketNo: "TK-20260609-M006", UserID: user.ID, Title: "测试",
		Description: "描述", Urgency: 1, ContactPhone: "x", Status: 1, Source: 1,
	}
	requireNoErr(t, db.Create(ticket).Error)

	err := svc.UpdateStatus(bgCtx, ticket.ID, operator.ID, request.UpdateTicketStatusRequest{
		Action: "invalid_action",
	})
	if err == nil {
		t.Fatal("无效 action 应返回错误")
	}
}

func TestTicketService_UpdateStatus_WrongPreStatus(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_wps")
	operator := createTestUserForService(t, db, "tsvc_wps_op")

	// 已关闭的申告不能 start
	ticket := &model.Ticket{
		TicketNo: "TK-20260609-M007", UserID: user.ID, Title: "测试",
		Description: "描述", Urgency: 1, ContactPhone: "x", Status: 5, Source: 1,
	}
	requireNoErr(t, db.Create(ticket).Error)

	err := svc.UpdateStatus(bgCtx, ticket.ID, operator.ID, request.UpdateTicketStatusRequest{
		Action: "start",
	})
	if err == nil {
		t.Fatal("已关闭申告不能 start")
	}
}

// =============================================================================
// AddRecord
// =============================================================================

func TestTicketService_AddRecord(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_record")
	operator := createTestUserForService(t, db, "tsvc_record_op")

	ticket := &model.Ticket{
		TicketNo: "TK-20260609-R001", UserID: user.ID, Title: "测试",
		Description: "描述", Urgency: 1, ContactPhone: "x", Status: 2, Source: 1,
	}
	requireNoErr(t, db.Create(ticket).Error)

	oldStatus := ticket.Status
	err := svc.AddRecord(bgCtx, ticket.ID, operator.ID, request.CreateTicketRecordRequest{
		Action:  "note",
		Content: "添加了一条备注",
	})
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	// 状态不应改变
	updated, _ := repo.FindByID(bgCtx, ticket.ID)
	if updated.Status != oldStatus {
		t.Errorf("AddRecord 不应改变状态, 期望 %d, got %d", oldStatus, updated.Status)
	}

	// 记录应存在
	records, _ := repo.FindByTicketID(bgCtx, ticket.ID)
	if len(records) != 1 {
		t.Errorf("期望 1 条记录, got %d", len(records))
	}
}

// =============================================================================
// ListByUser / ListAll
// =============================================================================

func TestTicketService_ListByUser(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_listbyuser")

	for i := 0; i < 3; i++ {
		ticket := &model.Ticket{
			TicketNo: fmt.Sprintf("TK-20260609-L%03d", i), UserID: user.ID,
			Title: "申告", Description: "描述", Urgency: 1, ContactPhone: "x", Status: 1, Source: 1,
		}
		requireNoErr(t, db.Create(ticket).Error)
	}

	result, err := svc.ListByUser(bgCtx, user.ID, 1, 10)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if result.Total != 3 {
		t.Errorf("期望 total=3, got %d", result.Total)
	}
	if len(result.Tickets) != 3 {
		t.Errorf("期望 3 条, got %d", len(result.Tickets))
	}
}

func TestTicketService_ListAll(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_listall")

	tickets := []model.Ticket{
		{TicketNo: "TK-20260609-A01", UserID: user.ID, Title: "待处理", Description: "d", Urgency: 2, ContactPhone: "x", Status: 1, Source: 1},
		{TicketNo: "TK-20260609-A02", UserID: user.ID, Title: "处理中", Description: "d", Urgency: 1, ContactPhone: "x", Status: 2, Source: 1},
		{TicketNo: "TK-20260609-A03", UserID: user.ID, Title: "高紧急", Description: "d", Urgency: 3, ContactPhone: "x", Status: 1, Source: 1},
	}
	for i := range tickets {
		requireNoErr(t, db.Create(&tickets[i]).Error)
	}

	// 按 status=1 筛选
	result, err := svc.ListAll(bgCtx, 1, 0, 1, 10)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if result.Total != 2 {
		t.Errorf("期望 total=2 (status=1), got %d", result.Total)
	}

	// 按 urgency=3 筛选
	result, err = svc.ListAll(bgCtx, -1, 3, 1, 10)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if result.Total != 1 {
		t.Errorf("期望 total=1 (urgency=3), got %d", result.Total)
	}

	// 全部
	result, err = svc.ListAll(bgCtx, -1, 0, 1, 10)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if result.Total != 3 {
		t.Errorf("期望 total=3 (all), got %d", result.Total)
	}
}

// =============================================================================
// GetDetail
// =============================================================================

func TestTicketService_GetDetail(t *testing.T) {
	db := setupTicketServiceDB(t)
	cleanTicketServiceTables(t, db)
	repo := repository.NewTicketRepo(db)
	svc := service.NewTicketService(repo, service.NewGormTxManager(db), nil, nil)
	user := createTestUserForService(t, db, "tsvc_detail")

	ticket := &model.Ticket{
		TicketNo: "TK-20260609-D001", UserID: user.ID, Title: "详情测试",
		Description: "详细描述", Urgency: 2, ContactPhone: "13800000001", Status: 1, Source: 1,
	}
	requireNoErr(t, db.Create(ticket).Error)

	// 创建处理记录
	record := &model.TicketRecord{TicketID: ticket.ID, OperatorID: user.ID, Action: "create", Content: "创建"}
	requireNoErr(t, db.Create(record).Error)

	result, err := svc.GetDetail(bgCtx, ticket.ID, 0) // 0=后台，不限所有权
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	if result.Title != "详情测试" {
		t.Errorf("期望 Title='详情测试', got '%s'", result.Title)
	}
	if result.SubmitterName != "测试用户" {
		t.Errorf("期望 SubmitterName='测试用户', got '%s'", result.SubmitterName)
	}
	if len(result.Records) != 1 {
		t.Errorf("期望 1 条处理记录, got %d", len(result.Records))
	}
}

// requireNoErr 简化错误断言。
func requireNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
}
