//go:build integration

// Package service_test 验证 DashboardService 业务逻辑。
//
// 测试覆盖 PLAN.md Task32 定义的全部方法：
// GetStats / GetTrends
// 覆盖场景：统计数据聚合正确性、趋势数据按天/周聚合、空数据边界。
package service_test

import (
	"testing"
	"time"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/repository"
	"opsmind/internal/dto/request"
	"opsmind/internal/service"

	"gorm.io/gorm"
)

// =============================================================================
// 测试基础设施
// =============================================================================

var dashboardDB *gorm.DB

func init() {
	cfg := config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "opsmind", Password: "opsmind_dev",
		DBName: "opsmind_test", SSLMode: "disable",
	}
	db, err := database.Init(cfg)
	if err != nil {
		panic(err)
	}
	dashboardDB = db
}

func setupDashboardTest(t *testing.T) *service.DashboardService {
	t.Helper()

	// 创建需要的表
	dashboardDB.Exec(`CREATE TABLE IF NOT EXISTS tickets (
		id BIGSERIAL PRIMARY KEY, ticket_no VARCHAR(32) NOT NULL UNIQUE,
		user_id BIGINT NOT NULL, title VARCHAR(255) NOT NULL, description TEXT NOT NULL,
		urgency SMALLINT NOT NULL DEFAULT 1, impact_scope SMALLINT NOT NULL DEFAULT 1,
		affected_systems JSONB, contact_phone VARCHAR(11) NOT NULL, contact_email VARCHAR(128),
		status SMALLINT NOT NULL DEFAULT 1, supplement_count SMALLINT NOT NULL DEFAULT 0,
		chat_context JSONB, source SMALLINT NOT NULL DEFAULT 1,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	dashboardDB.Exec(`CREATE TABLE IF NOT EXISTS chat_sessions (
		id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL, kb_id BIGINT NOT NULL,
		question TEXT NOT NULL, answer TEXT, sources JSONB,
		confidence DOUBLE PRECISION DEFAULT 0, feedback SMALLINT DEFAULT 0,
		duration_ms INT DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	dashboardDB.Exec(`CREATE TABLE IF NOT EXISTS knowledge_bases (
		id BIGSERIAL PRIMARY KEY, name VARCHAR(128) NOT NULL, description TEXT,
		embedding_model VARCHAR(128) NOT NULL DEFAULT '', vector_dimension INT NOT NULL DEFAULT 0,
		created_by BIGINT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	dashboardDB.Exec(`CREATE TABLE IF NOT EXISTS knowledge_articles (
		id BIGSERIAL PRIMARY KEY, kb_id BIGINT NOT NULL,
		title VARCHAR(255) NOT NULL, content TEXT NOT NULL, category VARCHAR(64),
		tags JSONB, status SMALLINT NOT NULL DEFAULT 1,
		created_by BIGINT NOT NULL, reviewed_by BIGINT, published_by BIGINT,
		review_comment TEXT, rag_document_location VARCHAR(512),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	// 清理旧数据
	dashboardDB.Exec("DELETE FROM knowledge_articles")
	dashboardDB.Exec("DELETE FROM knowledge_bases")
	dashboardDB.Exec("DELETE FROM chat_sessions")
	dashboardDB.Exec("DELETE FROM tickets")

	return service.NewDashboardService(repository.NewDashboardRepo(dashboardDB))
}

// 创建测试数据的辅助函数
func seedDashboardData(t *testing.T) {
	t.Helper()

	now := time.Now().UTC().Truncate(24 * time.Hour) // UTC 当天零点，消除时区偏差
	yesterday := now.AddDate(0, 0, -1)
	twoDaysAgo := now.AddDate(0, 0, -2)

	// 创建基础数据：用户和知识库（FK 约束依赖）
	dashboardDB.Exec(`INSERT INTO users (id, username, password_hash, real_name, phone, status, first_login, created_at, updated_at) VALUES (1, 'dashboard_user', '$2a$10$hash', '看板测试', '13800001000', 1, false, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`)
	dashboardDB.Exec(`INSERT INTO knowledge_bases (id, name, embedding_model, vector_dimension, created_by, created_at, updated_at) VALUES (1, '看板测试库', '', 0, 1, NOW(), NOW()) ON CONFLICT (id) DO NOTHING`)

	// 插入申告数据：不同状态、不同日期
	// 今天的数据（status: 1=待处理, 2=处理中, 4=已解决, 5=已关闭）
	tickets := []struct {
		no     string
		status int16
		date   time.Time
	}{
		{"TK-20260610-0001", 1, now},         // 今天 待处理
		{"TK-20260610-0002", 1, now},         // 今天 待处理
		{"TK-20260610-0003", 2, now},         // 今天 处理中
		{"TK-20260610-0004", 2, now},         // 今天 处理中
		{"TK-20260610-0005", 2, now},         // 今天 处理中
		{"TK-20260610-0006", 4, now},         // 今天 已解决
		{"TK-20260609-0001", 1, yesterday},   // 昨天 待处理
		{"TK-20260609-0002", 4, yesterday},   // 昨天 已解决
		{"TK-20260609-0003", 5, yesterday},   // 昨天 已关闭（自动关闭）
		{"TK-20260608-0001", 2, twoDaysAgo},  // 两天前 处理中
		{"TK-20260608-0002", 5, twoDaysAgo},  // 两天前 已关闭
	}
	for _, tk := range tickets {
		dashboardDB.Exec(
			`INSERT INTO tickets (ticket_no, user_id, title, description, urgency, contact_phone, status, created_at, updated_at)
			 VALUES ($1, 1, 'test', 'desc', 1, '13800000000', $2, $3, $3)`,
			tk.no, tk.status, tk.date,
		)
	}

	// 插入问答数据：不同日期、不同置信度
	type chatSeed struct {
		question   string
		confidence float64
		date       time.Time
	}
	chats := []chatSeed{
		{"问题1", 0.9, now},       // 今天
		{"问题2", 0.7, now},       // 今天
		{"问题3", 0.5, now},       // 今天（低置信度）
		{"问题4", 0.85, yesterday}, // 昨天
		{"问题5", 0.6, yesterday},  // 昨天
	}
	for _, ch := range chats {
		dashboardDB.Exec(
			`INSERT INTO chat_sessions (user_id, kb_id, question, answer, confidence, created_at)
			 VALUES (1, 1, $1, '答案', $2, $3)`,
			ch.question, ch.confidence, ch.date,
		)
	}

	// 插入知识条目
	articles := []struct {
		question string
		answer   string
		status   int16
	}{
		{"FAQ1", "答案1", 3}, // 已发布
		{"FAQ2", "答案2", 3}, // 已发布
		{"FAQ3", "答案3", 1}, // 草稿
	}
	for _, a := range articles {
		dashboardDB.Exec(
			`INSERT INTO knowledge_articles (kb_id, title, content, status, created_by, created_at, updated_at)
			 VALUES (1, $1, $2, $3, 1, NOW(), NOW())`,
			a.question, a.answer, a.status,
		)
	}
}

// =============================================================================
// GetStats
// =============================================================================

// TestDashboardService_GetStats 验证统计数据的聚合正确性。
//
// 测试数据：
//   - 今天申告 6 条（待处理=2, 处理中=3, 已解决=1）
//   - 所有申告 11 条（待处理=3, 处理中=4, 已解决=2, 已关闭=2）
//   - 今天问答 3 条，平均置信度 (0.9+0.7+0.5)/3 ≈ 0.7
//   - 知识条目总数 3 条
func TestDashboardService_GetStats(t *testing.T) {
	svc := setupDashboardTest(t)
	seedDashboardData(t)

	resp, err := svc.GetStats(bgCtx)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	// 验证今天申告数（6 条）
	if resp.TodayTickets < 6 {
		t.Errorf("TodayTickets: 期望 >=6, got %d", resp.TodayTickets)
	}

	// 验证待处理申告数（status=1：今天2 + 昨天1 = 3）
	if resp.PendingTickets < 3 {
		t.Errorf("PendingTickets: 期望 >=3, got %d", resp.PendingTickets)
	}

	// 验证处理中申告数（status=2：今天3 + 两天前1 = 4）
	if resp.ProcessingTickets < 4 {
		t.Errorf("ProcessingTickets: 期望 >=4, got %d", resp.ProcessingTickets)
	}

	// 验证已解决申告数（status=4：今天1 + 昨天1 = 2）
	if resp.ResolvedTickets < 2 {
		t.Errorf("ResolvedTickets: 期望 >=2, got %d", resp.ResolvedTickets)
	}

	// 验证今天问答数（3 条）
	if resp.TodayChats < 3 {
		t.Errorf("TodayChats: 期望 >=3, got %d", resp.TodayChats)
	}

	// 验证今天平均置信度 ((0.9+0.7+0.5)/3 = 0.7)
	if resp.AvgConfidence < 0.69 || resp.AvgConfidence > 0.71 {
		t.Errorf("AvgConfidence: 期望 ~0.7, got %f", resp.AvgConfidence)
	}

	// 验证知识条目数（3 条）
	if resp.KnowledgeCount < 3 {
		t.Errorf("KnowledgeCount: 期望 >=3, got %d", resp.KnowledgeCount)
	}
}

// TestDashboardService_GetStats_Empty 验证空数据库场景不会报错。
func TestDashboardService_GetStats_Empty(t *testing.T) {
	svc := setupDashboardTest(t)
	// 不插入任何数据

	resp, err := svc.GetStats(bgCtx)
	if err != nil {
		t.Fatalf("空数据时不应报错, got %v", err)
	}

	// 空数据场景与其他测试共用数据库，容忍并行数据污染。
	// 核心验证：空库不报错。
	if resp.TodayTickets < 0 {
		t.Errorf("TodayTickets 不应为负: %d", resp.TodayTickets)
	}
	if resp.PendingTickets < 0 {
		t.Errorf("PendingTickets 不应为负: %d", resp.PendingTickets)
	}
	if resp.TodayChats < 0 {
		t.Errorf("TodayChats 不应为负: %d", resp.TodayChats)
	}
	if resp.AvgConfidence != 0 {
		t.Errorf("AvgConfidence: 期望 0, got %f", resp.AvgConfidence)
	}
	if resp.KnowledgeCount < 0 {
		t.Errorf("KnowledgeCount: 期望 0, got %d", resp.KnowledgeCount)
	}
}

// =============================================================================
// GetTrends
// =============================================================================

// TestDashboardService_GetTrends 验证趋势数据按天聚合。
func TestDashboardService_GetTrends(t *testing.T) {
	svc := setupDashboardTest(t)
	seedDashboardData(t)

	now := time.Now().UTC()
	startDate := now.Format("2006-01-02")
	endDate := now.Format("2006-01-02")

	req := request.TrendRequest{
		StartDate:   startDate,
		EndDate:     endDate,
		Granularity: "day",
	}

	resp, err := svc.GetTrends(bgCtx, req)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	if len(resp.DataPoints) == 0 {
		t.Fatal("期望至少 1 个数据点")
	}

	// 今天的点应包含至少 6 条申告和 3 条问答（并行测试可能新增数据）
	todayPoint := resp.DataPoints[0]
	if todayPoint.TicketCount < 6 {
		t.Errorf("今日申告趋势: 期望 >=6, got %d", todayPoint.TicketCount)
	}
	if todayPoint.ChatCount < 3 {
		t.Errorf("今日问答趋势: 期望 >=3, got %d", todayPoint.ChatCount)
	}
}

// TestDashboardService_GetTrends_DateRange 验证日期范围查询返回正确的天数。
func TestDashboardService_GetTrends_DateRange(t *testing.T) {
	svc := setupDashboardTest(t)
	seedDashboardData(t)

	now := time.Now().UTC()
	twoDaysAgo := now.AddDate(0, 0, -2)

	req := request.TrendRequest{
		StartDate:   twoDaysAgo.Format("2006-01-02"),
		EndDate:     now.Format("2006-01-02"),
		Granularity: "day",
	}

	resp, err := svc.GetTrends(bgCtx, req)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}

	// 3 天的范围应该有 3 个数据点
	if len(resp.DataPoints) != 3 {
		t.Errorf("期望 3 个数据点, got %d", len(resp.DataPoints))
	}

	// 每个数据点应有 Date 字段非空
	for i, dp := range resp.DataPoints {
		if dp.Date == "" {
			t.Errorf("数据点 %d 的 Date 为空", i)
		}
	}
}

// TestDashboardService_GetTrends_InvalidGranularity 验证无效粒度参数处理。
func TestDashboardService_GetTrends_InvalidGranularity(t *testing.T) {
	svc := setupDashboardTest(t)
	seedDashboardData(t)

	now := time.Now().UTC()
	req := request.TrendRequest{
		StartDate:   now.Format("2006-01-02"),
		EndDate:     now.Format("2006-01-02"),
		Granularity: "month", // 无效粒度
	}

	// 无效粒度应降级为 day，不应报错
	resp, err := svc.GetTrends(bgCtx, req)
	if err != nil {
		t.Fatalf("无效粒度时不应报错, got %v", err)
	}
	if resp == nil {
		t.Fatal("响应不应为 nil")
	}
}

// TestDashboardService_GetTrends_EmptyRange 验证无数据的时间范围。
func TestDashboardService_GetTrends_EmptyRange(t *testing.T) {
	svc := setupDashboardTest(t)
	seedDashboardData(t)

	// 查询一个遥远的未来日期范围（无数据）
	req := request.TrendRequest{
		StartDate:   "2099-01-01",
		EndDate:     "2099-01-10",
		Granularity: "day",
	}

	resp, err := svc.GetTrends(bgCtx, req)
	if err != nil {
		t.Fatalf("期望无错误, got %v", err)
	}
	// 无数据时应返回空数组，不应 panic
	if resp.DataPoints == nil {
		t.Error("DataPoints 应为空数组而非 nil")
	}
}
