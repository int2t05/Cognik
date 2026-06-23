//go:build integration

// Package integration_test 验证 seed_demo.sql 演示数据的正确性。
//
// 测试覆盖演示数据场景：
//   - 预设角色数据完整性
//   - 预设用户账号和密码可用性
//   - 知识库和文章数据完整性
//   - 申告工单状态正确性
//
// 运行前需先加载演示数据：
//
//   make db-demo
//   go test -tags=integration -run TestSeedData ./tests/integration/ -v
package integration_test

import (
	"testing"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
	"opsmind/pkg/hash"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestSeedData_Roles 验证预设角色数据。
func TestSeedData_Roles(t *testing.T) {
	db := setupSeedDB(t)

	var roles []model.Role
	db.Order("id ASC").Find(&roles)
	// 种子数据检测：通过特定角色名判断是否已加载
	var seedRole model.Role
	if db.Where("name = ?", "系统管理员").First(&seedRole).Error != nil {
		t.Skip("本测试依赖种子数据，请先执行 make db-demo")
	}

	require.GreaterOrEqual(t, len(roles), 4, "至少应有 4 个预设角色")

	roleNames := make(map[string]bool)
	for _, r := range roles {
		roleNames[r.Name] = true
	}
	assert.True(t, roleNames["系统管理员"], "应有系统管理员角色")
	assert.True(t, roleNames["运维人员"], "应有运维人员角色")
	assert.True(t, roleNames["知识库管理员"], "应有知识库管理员角色")
	assert.True(t, roleNames["报障人"], "应有报障人角色")
	t.Logf("✅ 角色数据: %d 个角色", len(roles))
}

// TestSeedData_Users 验证预设用户账号。
func TestSeedData_Users(t *testing.T) {
	db := setupSeedDB(t)

	var users []model.User
	db.Order("id ASC").Find(&users)
	var seedUser model.User
	if db.Where("username = ?", "admin").First(&seedUser).Error != nil {
		t.Skip("本测试依赖种子数据，请先执行 make db-demo")
	}

	require.GreaterOrEqual(t, len(users), 6, "至少应有 6 个预设用户")

	// 验证 admin 账号
	var admin model.User
	err := db.Where("username = ?", "admin").First(&admin).Error
	require.NoError(t, err, "admin 账号应存在")
	assert.Equal(t, "系统管理员", admin.RealName)
	assert.Equal(t, int16(1), admin.Status, "admin 应为正常状态")
	// 验证密码哈希可校验
	assert.True(t, hash.CheckPassword(admin.PasswordHash, "Admin@123"),
		"admin 密码哈希应与 Admin@123 匹配")
	t.Logf("✅ admin 账号: username=%s, status=%d", admin.Username, admin.Status)

	// 验证报障人账号
	var reporter model.User
	err = db.Where("username = ?", "reporter1").First(&reporter).Error
	require.NoError(t, err, "reporter1 账号应存在")
	assert.True(t, reporter.FirstLogin, "reporter1 应为首次登录")
	t.Logf("✅ reporter1 账号: first_login=%v", reporter.FirstLogin)

	// 验证 reporter2 非首次登录
	var reporter2 model.User
	db.Where("username = ?", "reporter2").First(&reporter2)
	assert.False(t, reporter2.FirstLogin, "reporter2 不应是首次登录")
	t.Logf("✅ reporter2 账号: first_login=false")
}

// TestSeedData_UserRoles 验证用户-角色关联。
func TestSeedData_UserRoles(t *testing.T) {
	db := setupSeedDB(t)

	var userRoles []model.UserRole
	db.Find(&userRoles)
	var count int64
	db.Table("user_roles").Count(&count)
	if count == 0 {
		t.Skip("本测试依赖种子数据，请先执行 make db-demo")
	}

	assert.GreaterOrEqual(t, len(userRoles), 6, "至少应有 6 条用户-角色关联")
	t.Logf("✅ 用户-角色关联: %d 条", len(userRoles))

	// 验证 admin 有关联角色
	count = 0
	db.Table("user_roles").Where("user_id = ?", 1).Count(&count)
	assert.Greater(t, count, int64(0), "admin 应有关联角色")
	t.Logf("✅ admin 用户-角色关联: %d 条", count)
}

// TestSeedData_KnowledgeBase 验证知识库和文章数据。
func TestSeedData_KnowledgeBase(t *testing.T) {
	db := setupSeedDB(t)

	var kbs []model.KnowledgeBase
	db.Find(&kbs)
	var seedKB model.KnowledgeBase
	if db.Where("name = ?", "IT 运维 FAQ").First(&seedKB).Error != nil {
		t.Skip("本测试依赖种子数据，请先执行 make db-demo")
	}
	require.GreaterOrEqual(t, len(kbs), 1, "至少应有 1 个知识库")
	assert.Equal(t, "IT 运维 FAQ", kbs[0].Name)
	assert.Equal(t, "opsmind-it-ops", kbs[0].RAGWorkspaceSlug)
	t.Logf("✅ 知识库: name=%s, slug=%s", kbs[0].Name, kbs[0].RAGWorkspaceSlug)

	// 验证文章数据
	var articles []model.KnowledgeArticle
	db.Find(&articles)
	assert.GreaterOrEqual(t, len(articles), 5, "至少应有 5 条知识文章")

	// 验证各状态的文章
	statusCounts := map[int16]int{}
	for _, a := range articles {
		statusCounts[a.Status]++
	}
	assert.GreaterOrEqual(t, statusCounts[int16(4)], 3, "至少 3 篇已发布(status=4)")
	assert.GreaterOrEqual(t, statusCounts[int16(2)], 1, "至少 1 篇待审核(status=2)")
	assert.GreaterOrEqual(t, statusCounts[int16(1)], 1, "至少 1 篇草稿(status=1)")
	t.Logf("✅ 文章: %d 篇 (已发布=%d, 待审核=%d, 草稿=%d)",
		len(articles), statusCounts[4], statusCounts[2], statusCounts[1])

	// 验证 chunks
	var chunks []model.KnowledgeChunk
	db.Find(&chunks)
	assert.GreaterOrEqual(t, len(chunks), 5, "至少应有 5 个知识切片")
	t.Logf("✅ 知识切片: %d 个", len(chunks))
}

// TestSeedData_Tickets 验证申告工单数据。
func TestSeedData_Tickets(t *testing.T) {
	db := setupSeedDB(t)

	var tickets []model.Ticket
	db.Order("id ASC").Find(&tickets)
	var seedTicket model.Ticket
	if db.Where("ticket_no LIKE ?", "TK-DEMO%").First(&seedTicket).Error != nil {
		t.Skip("本测试依赖种子数据，请先执行 make db-demo")
	}
	require.GreaterOrEqual(t, len(tickets), 4, "至少应有 4 个申告工单")

	// 验证各状态工单存在
	statusCounts := map[int16]int{}
	for _, tk := range tickets {
		statusCounts[tk.Status]++
	}
	assert.GreaterOrEqual(t, statusCounts[int16(1)], 1, "应有待处理工单(status=1)")
	assert.GreaterOrEqual(t, statusCounts[int16(2)], 1, "应有处理中工单(status=2)")
	assert.GreaterOrEqual(t, statusCounts[int16(3)], 1, "应有需补充信息工单(status=3)")
	assert.GreaterOrEqual(t, statusCounts[int16(4)], 1, "应有已解决工单(status=4)")
	t.Logf("✅ 申告: %d 个 (待处理=%d, 处理中=%d, 需补充=%d, 已解决=%d)",
		len(tickets), statusCounts[1], statusCounts[2], statusCounts[3], statusCounts[4])

	// 验证处理记录
	var records []model.TicketRecord
	db.Find(&records)
	assert.GreaterOrEqual(t, len(records), 5, "至少应有 5 条处理记录")
	t.Logf("✅ 处理记录: %d 条", len(records))
}

// TestSeedData_Messages 验证站内消息。
func TestSeedData_Messages(t *testing.T) {
	db := setupSeedDB(t)

	var messages []model.Message
	db.Find(&messages)
	assert.GreaterOrEqual(t, len(messages), 3, "至少应有 3 条站内消息")

	// 验证有未读消息
	hasUnread := false
	for _, m := range messages {
		if !m.IsRead {
			hasUnread = true
			break
		}
	}
	assert.True(t, hasUnread, "应有未读消息")
	t.Logf("✅ 站内消息: %d 条", len(messages))
}

// setupSeedDB 连接测试数据库。
func setupSeedDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbCfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "opsmind",
		Password: "opsmind_dev",
		DBName:   "opsmind",
		SSLMode:  "disable",
	}
	db, err := database.Init(dbCfg)
	require.NoError(t, err, "初始化数据库失败")
	return db
}
