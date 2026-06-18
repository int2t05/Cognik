//go:build integration

// Package repository_test 验证 UserRepo 数据访问层。
//
// 测试覆盖 5 个核心方法：GetByID/GetByUsername/GetByPhone/ExistsByPhone/Create。
// 使用独立的 opsmind_test 数据库，每个测试用例通过清理保证隔离性。
package repository_test

import (
	"context"
	"testing"
	"time"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
	"opsmind/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupUserTestDB 初始化测试数据库连接并确保 users 表存在。
func setupUserTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbCfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "opsmind",
		Password: "opsmind_dev",
		DBName:   "opsmind_test",
		SSLMode:  "disable",
	}

	db, err := database.Init(dbCfg)
	if err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}

	// 确保 users 表存在
	err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
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
	)`).Error
	if err != nil {
		t.Fatalf("创建 users 表失败: %v", err)
	}

	return db
}

// cleanUsers 清理测试数据
func cleanUsers(t *testing.T, db *gorm.DB) {
	t.Helper()
	db.Exec("DELETE FROM users WHERE username LIKE 'test_%'")
}

// TestUserRepo_GetByID_Existing 按 ID 查询已存在的用户
func TestUserRepo_GetByID_Existing(t *testing.T) {
	db := setupUserTestDB(t)
	cleanUsers(t, db)
	repo := repository.NewUserRepo(db)

	now := time.Now()
	user := &model.User{
		Username:     "test_getbyid",
		PasswordHash: "$2a$10$hash",
		RealName:     "测试用户",
		Phone:        "13800000001",
		Email:        "test@example.com",
		Status:       1,
		FirstLogin:   true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	require.NoError(t, db.Create(user).Error)
	require.NotZero(t, user.ID)

	got, err := repo.GetByID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
	assert.Equal(t, "test_getbyid", got.Username)
	assert.Equal(t, "测试用户", got.RealName)
	assert.Equal(t, "13800000001", got.Phone)
	assert.Equal(t, "test@example.com", got.Email)
	assert.Equal(t, int16(1), got.Status)
	assert.True(t, got.FirstLogin)
}

// TestUserRepo_GetByID_NotFound 按 ID 查询不存在的用户
func TestUserRepo_GetByID_NotFound(t *testing.T) {
	db := setupUserTestDB(t)
	repo := repository.NewUserRepo(db)

	got, err := repo.GetByID(context.Background(), 999999)
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

// TestUserRepo_GetByUsername_Existing 按用户名查询已存在的用户
func TestUserRepo_GetByUsername_Existing(t *testing.T) {
	db := setupUserTestDB(t)
	cleanUsers(t, db)
	repo := repository.NewUserRepo(db)

	now := time.Now()
	user := &model.User{
		Username:     "test_getbyusername",
		PasswordHash: "$2a$10$hash",
		RealName:     "用户名查询",
		Phone:        "13800000002",
		Status:       1,
		FirstLogin:   true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	require.NoError(t, db.Create(user).Error)

	got, err := repo.GetByUsername(context.Background(), "test_getbyusername")
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
	assert.Equal(t, "test_getbyusername", got.Username)
}

// TestUserRepo_GetByUsername_NotFound 按用户名查询不存在的用户
func TestUserRepo_GetByUsername_NotFound(t *testing.T) {
	db := setupUserTestDB(t)
	repo := repository.NewUserRepo(db)

	got, err := repo.GetByUsername(context.Background(), "nonexistent_user_xyz")
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

// TestUserRepo_GetByPhone_Existing 按手机号查询已存在的用户
func TestUserRepo_GetByPhone_Existing(t *testing.T) {
	db := setupUserTestDB(t)
	cleanUsers(t, db)
	repo := repository.NewUserRepo(db)

	now := time.Now()
	user := &model.User{
		Username:     "test_getbyphone",
		PasswordHash: "$2a$10$hash",
		RealName:     "手机查询",
		Phone:        "13800000003",
		Status:       1,
		FirstLogin:   true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	require.NoError(t, db.Create(user).Error)

	got, err := repo.GetByPhone(context.Background(), "13800000003")
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
	assert.Equal(t, "13800000003", got.Phone)
}

// TestUserRepo_GetByPhone_NotFound 按手机号查询不存在的用户
func TestUserRepo_GetByPhone_NotFound(t *testing.T) {
	db := setupUserTestDB(t)
	repo := repository.NewUserRepo(db)

	got, err := repo.GetByPhone(context.Background(), "99999999999")
	assert.Error(t, err)
	assert.Nil(t, got)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

// TestUserRepo_ExistsByPhone_True 手机号存在时返回 true
func TestUserRepo_ExistsByPhone_True(t *testing.T) {
	db := setupUserTestDB(t)
	cleanUsers(t, db)
	repo := repository.NewUserRepo(db)

	now := time.Now()
	user := &model.User{
		Username:     "test_existsphone",
		PasswordHash: "$2a$10$hash",
		RealName:     "存在检测",
		Phone:        "13800000004",
		Status:       1,
		FirstLogin:   true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	require.NoError(t, db.Create(user).Error)

	exists, err := repo.ExistsByPhone(context.Background(), "13800000004")
	require.NoError(t, err)
	assert.True(t, exists)
}

// TestUserRepo_ExistsByPhone_False 手机号不存在时返回 false
func TestUserRepo_ExistsByPhone_False(t *testing.T) {
	db := setupUserTestDB(t)
	repo := repository.NewUserRepo(db)

	exists, err := repo.ExistsByPhone(context.Background(), "99999999999")
	require.NoError(t, err)
	assert.False(t, exists)
}

// TestUserRepo_Create 新增用户
func TestUserRepo_Create(t *testing.T) {
	db := setupUserTestDB(t)
	cleanUsers(t, db)
	repo := repository.NewUserRepo(db)

	now := time.Now()
	user := &model.User{
		Username:     "test_create",
		PasswordHash: "$2a$10$hash_value_here",
		RealName:     "新用户",
		Phone:        "13800000005",
		Email:        "new@example.com",
		Status:       1,
		FirstLogin:   true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err := repo.Create(context.Background(), user)
	require.NoError(t, err)
	assert.NotZero(t, user.ID, "Create 后应自动填充 ID")

	// 验证可查回
	got, err := repo.GetByID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, "test_create", got.Username)
	assert.Equal(t, "新用户", got.RealName)
	assert.Equal(t, "13800000005", got.Phone)
	assert.Equal(t, "new@example.com", got.Email)
}

// TestUserRepo_Create_DuplicateUsername 用户名重复时报错
func TestUserRepo_Create_DuplicateUsername(t *testing.T) {
	db := setupUserTestDB(t)
	cleanUsers(t, db)
	repo := repository.NewUserRepo(db)

	now := time.Now()
	user1 := &model.User{
		Username:     "test_dup_user",
		PasswordHash: "$2a$10$hash1",
		RealName:     "用户1",
		Phone:        "13800000010",
		Status:       1,
		FirstLogin:   true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	require.NoError(t, repo.Create(context.Background(), user1))

	user2 := &model.User{
		Username:     "test_dup_user", // 同名
		PasswordHash: "$2a$10$hash2",
		RealName:     "用户2",
		Phone:        "13800000011",
		Status:       1,
		FirstLogin:   true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err := repo.Create(context.Background(), user2)
	assert.Error(t, err, "用户名重复应返回错误")
}
