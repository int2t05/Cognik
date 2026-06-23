//go:build integration

// Package integration_test 验证 seed_essential.sql 必要数据的正确性。
//
// 测试覆盖必要数据场景：
//   - 预设角色数据完整性
//   - 预设用户账号和密码可用性
//   - 用户-角色关联
//   - LLM 配置存在性
//   - 系统配置存在性
//
// 运行前需先加载必要数据：
//
//	make db-seed
//	go test -tags=integration -run TestSeedData ./tests/integration/ -v
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
		t.Skip("本测试依赖种子数据，请先执行 make db-seed")
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
		t.Skip("本测试依赖种子数据，请先执行 make db-seed")
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
		t.Skip("本测试依赖种子数据，请先执行 make db-seed")
	}

	assert.GreaterOrEqual(t, len(userRoles), 6, "至少应有 6 条用户-角色关联")
	t.Logf("✅ 用户-角色关联: %d 条", len(userRoles))

	// 验证 admin 有关联角色
	count = 0
	db.Table("user_roles").Where("user_id = ?", 1).Count(&count)
	assert.Greater(t, count, int64(0), "admin 应有关联角色")
	t.Logf("✅ admin 用户-角色关联: %d 条", count)
}

// TestSeedData_LLMConfig 验证 LLM 配置数据。
func TestSeedData_LLMConfig(t *testing.T) {
	db := setupSeedDB(t)

	var configs []model.LlmConfig
	db.Find(&configs)
	if len(configs) == 0 {
		t.Skip("本测试依赖种子数据，请先执行 make db-seed")
	}

	assert.GreaterOrEqual(t, len(configs), 2, "至少应有 2 条 LLM 配置")

	// 验证存在默认配置
	hasDefault := false
	for _, c := range configs {
		if c.IsDefault {
			hasDefault = true
			t.Logf("✅ 默认配置: name=%s, llm=%s, embedding=%s, provider_type=%d",
				c.Name, c.LLMModel, c.EmbeddingModel, c.ProviderType)
			break
		}
	}
	assert.True(t, hasDefault, "应存在一条 is_default=true 的配置")

	// 验证 llama.cpp 本地配置存在
	var localCfg model.LlmConfig
	err := db.Where("provider_type = ?", 1).First(&localCfg).Error
	require.NoError(t, err, "应有 llama.cpp 本地配置 (provider_type=1)")
	assert.NotEmpty(t, localCfg.LLMModel, "llm_model 不应为空")
	assert.NotEmpty(t, localCfg.EmbeddingModel, "embedding_model 不应为空")
	t.Logf("✅ llama.cpp 配置: llm=%s, embedding=%s", localCfg.LLMModel, localCfg.EmbeddingModel)
}

// TestSeedData_Menus 验证菜单数据。
func TestSeedData_Menus(t *testing.T) {
	db := setupSeedDB(t)

	var menus []model.Menu
	db.Order("id ASC").Find(&menus)
	if len(menus) == 0 {
		t.Skip("本测试依赖种子数据，请先执行 make db-seed")
	}

	assert.GreaterOrEqual(t, len(menus), 9, "至少应有 9 个菜单")
	t.Logf("✅ 菜单数据: %d 个", len(menus))
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
