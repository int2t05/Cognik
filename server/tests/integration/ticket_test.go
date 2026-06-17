//go:build integration

// Package integration_test 验证申告模块的端到端完整流程。
//
// 测试覆盖 PLAN.md Task36 定义的场景：
//   - 完整状态机流转：创建→开始处理→需补充信息→补充信息→解决→回访→关闭
//   - request_info 超过 3 次被拒绝
//   - 7 天自动关闭
//   - 申告编号生成验证
//
// 使用真实数据库 opsmind_test，不 mock 外部服务。
package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/dto/request"
	"opsmind/internal/handler"
	"opsmind/internal/middleware"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// =============================================================================
// 测试环境
// =============================================================================

// ticketIntEnv 封装申告集成测试环境。
type ticketIntEnv struct {
	r              *gin.Engine
	db             *gorm.DB
	repo           *repository.TicketRepo
	svc            *service.TicketService
	submitterID    int64 // 报障人用户 ID（用于测试中创建申告）
	operatorID     int64 // 运维人员用户 ID
}

// setupTicketIntegration 创建申告集成测试环境。
func setupTicketIntegration(t *testing.T) *ticketIntEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbCfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "opsmind",
		Password: "opsmind_dev",
		DBName:   "opsmind_test",
		SSLMode:  "disable",
	}
	db, err := database.Init(dbCfg)
	require.NoError(t, err, "初始化数据库失败")

	// 建表
	db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY,
		username VARCHAR(64) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL,
		real_name VARCHAR(64) NOT NULL,
		phone VARCHAR(11) NOT NULL,
		email VARCHAR(128),
		status SMALLINT NOT NULL DEFAULT 1,
		first_login BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS tickets (
		id BIGSERIAL PRIMARY KEY,
		ticket_no VARCHAR(32) NOT NULL UNIQUE,
		user_id BIGINT NOT NULL,
		title VARCHAR(255) NOT NULL,
		description TEXT NOT NULL,
		urgency SMALLINT NOT NULL,
		impact_scope SMALLINT DEFAULT 1,
		affected_systems JSONB,
		contact_phone VARCHAR(11) NOT NULL,
		contact_email VARCHAR(128),
		status SMALLINT NOT NULL DEFAULT 1,
		supplement_count SMALLINT NOT NULL DEFAULT 0,
		chat_context JSONB,
		source SMALLINT NOT NULL DEFAULT 1,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	db.Exec(`CREATE TABLE IF NOT EXISTS ticket_records (
		id BIGSERIAL PRIMARY KEY,
		ticket_id BIGINT NOT NULL,
		operator_id BIGINT NOT NULL,
		action VARCHAR(32) NOT NULL,
		content TEXT,
		detail JSONB,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	// 清理
	db.Exec("DELETE FROM ticket_records")
	db.Exec("DELETE FROM tickets")
	db.Exec("DELETE FROM users")

	// 创建测试用户（提交申告的用户和运维操作用户）
	now := time.Now()
	submitter := &model.User{
		Username: "itg_ticket_user", PasswordHash: "$2a$10$hash",
		RealName: "报障用户", Phone: "13800002001", Status: 1, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, db.Create(submitter).Error)

	operator := &model.User{
		Username: "itg_ticket_op", PasswordHash: "$2a$10$hash",
		RealName: "运维人员", Phone: "13800002002", Status: 1, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, db.Create(operator).Error)

	// 组装依赖链
	ticketRepo := repository.NewTicketRepo(db)
	ticketSvc := service.NewTicketService(ticketRepo, service.NewGormTxManager(db), nil)
	ticketH := handler.NewTicketHandler(ticketSvc, nil)

	// 路由
	r := gin.New()
	r.Use(middleware.RequestID())

	// 门户端路由（模拟报障人 user_id=1）
	portal := r.Group("/api/v1/portal")
	portal.Use(func(c *gin.Context) {
		c.Set("currentUser", map[string]interface{}{"user_id": float64(submitter.ID)})
		c.Set("userID", submitter.ID)
		c.Next()
	})
	{
		portal.POST("/tickets", ticketH.CreateTicket)
		portal.GET("/tickets", ticketH.ListByUser)
		portal.GET("/tickets/:id", ticketH.GetDetail)
		portal.PATCH("/tickets/:id/supplement", ticketH.SupplementTicket)
	}

	// 后台路由（模拟运维人员 user_id=2）
	admin := r.Group("/api/v1/admin")
	admin.Use(func(c *gin.Context) {
		c.Set("currentUser", map[string]interface{}{
			"user_id":  float64(operator.ID),
			"username": "itg_ticket_op",
			"roles":    []interface{}{"admin"},
		})
		c.Set("userID", operator.ID)
		c.Next()
	})
	{
		admin.GET("/tickets", ticketH.ListAll)
		admin.GET("/tickets/:id", ticketH.GetDetail)
		admin.PATCH("/tickets/:id/status", ticketH.UpdateStatus)
		admin.POST("/tickets/:id/records", ticketH.AddRecord)
	}

	return &ticketIntEnv{r: r, db: db, repo: ticketRepo, svc: ticketSvc,
		submitterID: submitter.ID, operatorID: operator.ID}
}

// createTicketViaAPI 通过 API 创建申告，返回 ticket_id 和 ticket_no。
//
// 注意：当前 CreateTicket handler 返回 data=nil（不返回创建的 ID），
// 因此创建后从数据库查询最后插入的记录来获取 ID。
func createTicketViaAPI(t *testing.T, env *ticketIntEnv) (int64, string) {
	t.Helper()

	body := request.CreateTicketRequest{
		Title:        "办公区网络频繁断开",
		Description:  "从今天上午开始，3 楼办公区网络每隔 10 分钟断开一次",
		Urgency:      2,
		ImpactScope:  1,
		ContactPhone: "13800002001",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/portal/tickets", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.r.ServeHTTP(w, req)

	require.Equal(t, 200, w.Code, "创建申答应返回 200")

	var resp struct {
		Code int `json:"code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code, "创建申告业务码应为 0")

	// 从数据库获取最新创建的申告（handler 不返回 ID）
	var ticket model.Ticket
	env.db.Order("id desc").First(&ticket)
	return ticket.ID, ticket.TicketNo
}

// getTicketViaAPI 通过 API 获取申告详情。
func getTicketViaAPI(t *testing.T, env *ticketIntEnv, id int64, isAdmin bool) map[string]interface{} {
	t.Helper()

	prefix := "/api/v1/portal"
	if isAdmin {
		prefix = "/api/v1/admin"
	}

	req := httptest.NewRequest("GET", fmt.Sprintf("%s/tickets/%d", prefix, id), nil)
	w := httptest.NewRecorder()
	env.r.ServeHTTP(w, req)

	require.Equal(t, 200, w.Code, "获取详情应返回 200")

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp["data"].(map[string]interface{})
}

// updateTicketStatus 通过 API 更新申告状态。
func updateTicketStatus(t *testing.T, env *ticketIntEnv, id int64, action string) {
	t.Helper()

	body := request.UpdateTicketStatusRequest{
		Action: action,
		Result: "操作结果: " + action,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PATCH",
		fmt.Sprintf("/api/v1/admin/tickets/%d/status", id), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.r.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	code, _ := resp["code"].(float64)
	require.Equal(t, float64(0), code,
		"状态操作 '%s' 应成功, 响应: %s", action, w.Body.String())
}

// =============================================================================
// 完整状态机流转测试
// =============================================================================

// TestTicketIntegration_FullStateMachine 验证完整申告状态机流转。
//
// 流程：创建(待处理=1) → 开始处理(2) → 需补充信息(3) →
// 补充信息(2) → 解决(4) → 关闭(5)
func TestTicketIntegration_FullStateMachine(t *testing.T) {
	env := setupTicketIntegration(t)

	// 1. 报障人创建申告
	id, ticketNo := createTicketViaAPI(t, env)
	assert.NotEmpty(t, ticketNo, "应生成申告编号")
	assert.Contains(t, ticketNo, "TK-", "申告编号应以 TK- 开头")
	t.Logf("✅ 步骤1: 申告创建成功, ID=%d, 编号=%s", id, ticketNo)

	// 验证初始状态
	detail := getTicketViaAPI(t, env, id, true)
	assert.Equal(t, float64(1), detail["status"], "初始状态应为 1(待处理)")
	t.Logf("✅ 初始状态=待处理(1)")

	// 2. 运维人员开始处理 → status: 1→2
	updateTicketStatus(t, env, id, "start")
	detail = getTicketViaAPI(t, env, id, true)
	assert.Equal(t, float64(2), detail["status"], "开始处理后状态应为 2(处理中)")
	t.Logf("✅ 步骤2: 开始处理 → 状态=处理中(2)")

	// 3. 运维人员请求补充信息 → status: 2→3
	updateTicketStatus(t, env, id, "request_info")
	detail = getTicketViaAPI(t, env, id, true)
	assert.Equal(t, float64(3), detail["status"], "请求补充信息后状态应为 3(需补充信息)")
	t.Logf("✅ 步骤3: 请求补充信息 → 状态=需补充信息(3)")

	// 4. 报障人补充信息 → status: 3→2
	supplementBody := request.SupplementTicketRequest{
		Content: "补充说明：故障集中在 3 楼东侧，西侧正常",
	}
	jsonBody, _ := json.Marshal(supplementBody)
	req := httptest.NewRequest("PATCH",
		fmt.Sprintf("/api/v1/portal/tickets/%d/supplement", id), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code, "补充信息应返回 200")
	var suppResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &suppResp)
	assert.Equal(t, float64(0), suppResp["code"], "补充信息业务码应为 0")

	detail = getTicketViaAPI(t, env, id, true)
	assert.Equal(t, float64(2), detail["status"], "补充信息后状态应回到 2(处理中)")
	t.Logf("✅ 步骤4: 补充信息 → 状态回到=处理中(2)")

	// 5. 运维人员解决 → status: 2→4
	updateTicketStatus(t, env, id, "resolve")
	detail = getTicketViaAPI(t, env, id, true)
	assert.Equal(t, float64(4), detail["status"], "解决后状态应为 4(已解决)")
	t.Logf("✅ 步骤5: 解决 → 状态=已解决(4)")

	// 6. 运维人员关闭 → status: 4→5
	updateTicketStatus(t, env, id, "close")
	detail = getTicketViaAPI(t, env, id, true)
	assert.Equal(t, float64(5), detail["status"], "关闭后状态应为 5(已关闭)")
	t.Logf("✅ 步骤6: 关闭 → 状态=已关闭(5)")

	// 7. 验证处理记录时间线（应有至少 4 条记录：start/request_info/resolve/close）
	records, ok := detail["ticket_records"].([]interface{})
	if ok {
		assert.GreaterOrEqual(t, len(records), 4, "至少应有 4 条处理记录")
		t.Logf("✅ 处理记录: %d 条", len(records))
	}
}

// =============================================================================
// request_info 超过 3 次被拒绝
// =============================================================================

// TestTicketIntegration_SupplementLimitReached 验证补充信息超过 3 次被拒绝。
//
// 业务规则：每个申告最多请求补充信息 3 次，
// 超过后应拒绝再次 request_info 操作。
func TestTicketIntegration_SupplementLimitReached(t *testing.T) {
	env := setupTicketIntegration(t)

	// 1. 创建申告
	id, _ := createTicketViaAPI(t, env)

	// 2. 开始处理
	updateTicketStatus(t, env, id, "start")

	// 3. 循环 request_info → supplement 共 3 次
	for i := 1; i <= 3; i++ {
		// request_info → status 2→3
		updateTicketStatus(t, env, id, "request_info")

		// supplement → status 3→2
		supplementBody := request.SupplementTicketRequest{
			Content: fmt.Sprintf("第 %d 次补充信息", i),
		}
		jsonBody, _ := json.Marshal(supplementBody)
		req := httptest.NewRequest("PATCH",
			fmt.Sprintf("/api/v1/portal/tickets/%d/supplement", id), bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		env.r.ServeHTTP(w, req)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		require.Equal(t, float64(0), resp["code"],
			"第 %d 次补充信息应成功", i)
	}
	t.Logf("✅ 前 3 次补充信息均成功")

	// 4. 第 4 次 request_info → 应被拒绝（已满 3 次）
	body := request.UpdateTicketStatusRequest{
		Action: "request_info",
		Result: "第 4 次请求补充",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("PATCH",
		fmt.Sprintf("/api/v1/admin/tickets/%d/status", id), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.r.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotEqual(t, float64(0), resp["code"],
		"超过 3 次 request_info 应返回错误码")
	t.Logf("✅ 第 4 次 request_info 被拒绝 (code=%v)", resp["code"])

	// 验证状态仍为处理中(2)
	detail := getTicketViaAPI(t, env, id, true)
	assert.Equal(t, float64(2), detail["status"], "超过 3 次后状态应保持处理中(2)")

	// 验证 supplement_count 为 3
	var ticket model.Ticket
	env.db.First(&ticket, id)
	assert.Equal(t, int16(3), ticket.SupplementCount, "supplement_count 应为 3")
	t.Logf("✅ supplement_count=3，状态保持处理中(2)")
}

// =============================================================================
// 7 天自动关闭
// =============================================================================

// TestTicketIntegration_AutoClose 验证 7 天自动关闭逻辑。
//
// 创建 7 天前的申告，验证调度器 AutoCloseTickets 将其关闭。
func TestTicketIntegration_AutoClose(t *testing.T) {
	env := setupTicketIntegration(t)

	// 1. 创建一个 8 天前的申告（模拟过期）
	oldTicket := &model.Ticket{
		TicketNo:     "TK-20260602-A001",
		UserID:       env.submitterID,
		Title:        "过期的申告",
		Description:  "8 天前创建的申告",
		Urgency:      1,
		ContactPhone: "13800000001",
		Status:       1, // 待处理
		Source:       1,
		CreatedAt:    time.Now().Add(-8 * 24 * time.Hour),
		UpdatedAt:    time.Now().Add(-8 * 24 * time.Hour),
	}
	require.NoError(t, env.db.Create(oldTicket).Error)

	// 2. 再创建一个 1 小时前的新申告（不应被关闭）
	newTicket := &model.Ticket{
		TicketNo:     "TK-20260610-N001",
		UserID:       env.submitterID,
		Title:        "新的申告",
		Description:  "1 小时前创建的申告",
		Urgency:      1,
		ContactPhone: "13800000001",
		Status:       1,
		Source:       1,
		CreatedAt:    time.Now().Add(-1 * time.Hour),
		UpdatedAt:    time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, env.db.Create(newTicket).Error)

	// 3. 执行自动关闭（通过 service 的 AutoCloseTickets 逻辑）
	ids, err := env.repo.AutoCloseTickets(time.Now().Add(-7 * 24 * time.Hour))
	require.NoError(t, err)

	assert.Equal(t, 1, len(ids), "应关闭 1 个过期申告（仅 8 天前的）")
	t.Logf("✅ 自动关闭: %d 个申告被关闭", len(ids))

	// 4. 验证旧申告状态变为已关闭(5)
	var checkOld model.Ticket
	env.db.First(&checkOld, oldTicket.ID)
	assert.Equal(t, int16(5), checkOld.Status, "8 天前申告状态应为 5(已关闭)")
	t.Logf("✅ 8 天前申告已关闭 (status=5)")

	// 5. 验证新申告状态未变
	var checkNew model.Ticket
	env.db.First(&checkNew, newTicket.ID)
	assert.Equal(t, int16(1), checkNew.Status, "新申告状态应保持不变(1)")
	t.Logf("✅ 新申告未被关闭 (status=1)")
}

// =============================================================================
// 申告编号生成
// =============================================================================

// TestTicketIntegration_TicketNoGeneration 验证申告编号格式和递增。
func TestTicketIntegration_TicketNoGeneration(t *testing.T) {
	env := setupTicketIntegration(t)

	// 创建两个申告，验证编号递增
	id1, no1 := createTicketViaAPI(t, env)
	id2, no2 := createTicketViaAPI(t, env)

	// 验证格式: TK-YYYYMMDD-XXXX
	assert.Contains(t, no1, "TK-", "编号应以 TK- 开头")
	assert.Contains(t, no2, "TK-", "编号应以 TK- 开头")

	// 验证日期部分相同（同一天）
	datePart := time.Now().Format("20060102")
	assert.Contains(t, no1, datePart, "编号应包含当前日期")
	assert.Contains(t, no2, datePart, "编号应包含当前日期")

	// 验证序号递增
	assert.NotEqual(t, no1, no2, "两个申告编号应不同")
	t.Logf("✅ 申告编号: #1=%s (id=%d), #2=%s (id=%d)", no1, id1, no2, id2)
}

// =============================================================================
// 补充信息仅申告人可操作
// =============================================================================

// TestTicketIntegration_SupplementOnlyByOwner 验证补充信息仅申告人可操作。
func TestTicketIntegration_SupplementOnlyByOwner(t *testing.T) {
	env := setupTicketIntegration(t)

	// 创建申告并进入"需补充信息"状态
	id, _ := createTicketViaAPI(t, env)
	updateTicketStatus(t, env, id, "start")
	updateTicketStatus(t, env, id, "request_info")

	// 验证当前状态为 3
	detail := getTicketViaAPI(t, env, id, true)
	assert.Equal(t, float64(3), detail["status"])

	// 模拟另一个人（非申告人）尝试补充信息
	// 在 portal 路由中，中间件固定注入 user_id=1（提交者），
	// 补充信息时 service 会校验 user_id 是否为申告人。
	// 这里直接通过 service 层验证校验逻辑
	err := env.svc.SupplementTicket(id, 999, request.SupplementTicketRequest{
		Content: "其他人补充",
	})
	assert.Error(t, err, "非申告人补充信息应失败")
	t.Logf("✅ 非申告人补充信息被拒绝: %v", err)

	// 确认状态未变
	detail = getTicketViaAPI(t, env, id, true)
	assert.Equal(t, float64(3), detail["status"], "状态不应被修改")
}
