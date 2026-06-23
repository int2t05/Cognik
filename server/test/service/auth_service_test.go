//go:build integration

// Package service_test 验证 AuthService 认证业务逻辑。
//
// 测试覆盖 PLAN.md T11 定义的场景：登录成功、密码错误、账号冻结、
// 刷新令牌、修改密码（旧密码错误、新密码不符合策略）。
// 使用真实数据库集成测试，通过 seed 数据保证用户存在。
package service_test

import (
	"context"
	"testing"
	"time"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"
	"opsmind/pkg/hash"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var bgCtx = context.Background()

// testJWTConfig 返回测试用 JWT 配置，与 config.yaml 默认值一致。
func testJWTConfig() config.JWTConfig {
	return config.JWTConfig{
		Secret:        "test_secret_key_2024",
		AccessExpire:  2 * time.Hour,
		RefreshExpire: 168 * time.Hour,
	}
}

// setupAuthTestDB 初始化测试数据库并确保 users 表存在。
func setupAuthTestDB(t *testing.T) *gorm.DB {
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

	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
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

// seedTestUser 创建或更新测试用户，返回用户记录。
func seedTestUser(t *testing.T, db *gorm.DB, username, password, phone string, status int16) *model.User {
	t.Helper()

	hashed, err := hash.HashPassword(password)
	require.NoError(t, err, "密码哈希失败")

	user := &model.User{
		Username:     username,
		PasswordHash: hashed,
		RealName:     "测试用户",
		Phone:        phone,
		Status:       status,
		FirstLogin:   true,
	}

	// 先清理再插入
	db.Where("username = ?", username).Delete(&model.User{})
	require.NoError(t, db.Create(user).Error)
	return user
}

// TestAuthService_Login_Success 验证正常登录。
func TestAuthService_Login_Success(t *testing.T) {
	db := setupAuthTestDB(t)
	repo := repository.NewUserRepo(db)
	svc := service.NewAuthService(repo, nil, db, testJWTConfig())

	seedTestUser(t, db, "test_auth_login", "Test@1234", "13800001001", 1)

	resp, err := svc.Login(bgCtx, "test_auth_login", "Test@1234")
	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken, "应返回 access_token")
	assert.NotEmpty(t, resp.RefreshToken, "应返回 refresh_token")
	assert.Equal(t, "test_auth_login", resp.User.Username)
	assert.Equal(t, "测试用户", resp.User.RealName)
	assert.False(t, resp.User.FirstLogin, "首次登录后 first_login 应设为 false")
}

// TestAuthService_Login_WrongPassword 验证密码错误。
func TestAuthService_Login_WrongPassword(t *testing.T) {
	db := setupAuthTestDB(t)
	repo := repository.NewUserRepo(db)
	svc := service.NewAuthService(repo, nil, db, testJWTConfig())

	seedTestUser(t, db, "test_auth_wrong", "Test@1234", "13800001002", 1)

	resp, err := svc.Login(bgCtx, "test_auth_wrong", "WrongPassword@1")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, 10003, err.(service.AppError).Code, "密码错误应返回 10003")
}

// TestAuthService_Login_FrozenAccount 验证冻结账号登录。
func TestAuthService_Login_FrozenAccount(t *testing.T) {
	db := setupAuthTestDB(t)
	repo := repository.NewUserRepo(db)
	svc := service.NewAuthService(repo, nil, db, testJWTConfig())

	seedTestUser(t, db, "test_auth_frozen", "Test@1234", "13800001003", 2) // status=2 冻结

	resp, err := svc.Login(bgCtx, "test_auth_frozen", "Test@1234")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, 10002, err.(service.AppError).Code, "冻结账号应返回 10002")
}

// TestAuthService_Login_UserNotFound 验证用户不存在。
func TestAuthService_Login_UserNotFound(t *testing.T) {
	db := setupAuthTestDB(t)
	repo := repository.NewUserRepo(db)
	svc := service.NewAuthService(repo, nil, db, testJWTConfig())

	resp, err := svc.Login(bgCtx, "nonexistent_xyz", "Test@1234")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, 10003, err.(service.AppError).Code)
}

// TestAuthService_RefreshToken_Success 验证刷新令牌。
func TestAuthService_RefreshToken_Success(t *testing.T) {
	db := setupAuthTestDB(t)
	repo := repository.NewUserRepo(db)
	svc := service.NewAuthService(repo, nil, db, testJWTConfig())

	seedTestUser(t, db, "test_auth_refresh", "Test@1234", "13800001004", 1)

	// 先登录拿到 refresh_token
	loginResp, err := svc.Login(bgCtx, "test_auth_refresh", "Test@1234")
	require.NoError(t, err)

	// 刷新
	refreshResp, err := svc.RefreshToken(bgCtx, loginResp.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, refreshResp.AccessToken)
	assert.NotEmpty(t, refreshResp.RefreshToken)
	assert.Equal(t, "test_auth_refresh", refreshResp.User.Username)
}

// TestAuthService_RefreshToken_InvalidToken 验证无效刷新令牌。
func TestAuthService_RefreshToken_InvalidToken(t *testing.T) {
	db := setupAuthTestDB(t)
	repo := repository.NewUserRepo(db)
	svc := service.NewAuthService(repo, nil, db, testJWTConfig())

	resp, err := svc.RefreshToken(bgCtx, "invalid_token_xyz")
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, 10001, err.(service.AppError).Code)
}

// TestAuthService_ChangePassword_Success 验证正常修改密码。
func TestAuthService_ChangePassword_Success(t *testing.T) {
	db := setupAuthTestDB(t)
	repo := repository.NewUserRepo(db)
	svc := service.NewAuthService(repo, nil, db, testJWTConfig())

	user := seedTestUser(t, db, "test_auth_chpwd", "Test@1234", "13800001005", 1)

	err := svc.ChangePassword(bgCtx,user.ID, "Test@1234", "NewPass@123")
	assert.NoError(t, err)

	// 验证新密码可用
	resp, err := svc.Login(bgCtx, "test_auth_chpwd", "NewPass@123")
	require.NoError(t, err)
	assert.False(t, resp.User.FirstLogin, "修改密码后 first_login 应为 false")
}

// TestAuthService_ChangePassword_WrongOldPassword 验证旧密码错误。
func TestAuthService_ChangePassword_WrongOldPassword(t *testing.T) {
	db := setupAuthTestDB(t)
	repo := repository.NewUserRepo(db)
	svc := service.NewAuthService(repo, nil, db, testJWTConfig())

	user := seedTestUser(t, db, "test_auth_chpwd_old", "Test@1234", "13800001006", 1)

	err := svc.ChangePassword(bgCtx,user.ID, "WrongOld@123", "NewPass@123")
	assert.Error(t, err)
	assert.Equal(t, 10003, err.(service.AppError).Code, "旧密码错误应返回 10003")
}

// TestAuthService_ChangePassword_WeakNewPassword 验证新密码不符合策略。
func TestAuthService_ChangePassword_WeakNewPassword(t *testing.T) {
	db := setupAuthTestDB(t)
	repo := repository.NewUserRepo(db)
	svc := service.NewAuthService(repo, nil, db, testJWTConfig())

	user := seedTestUser(t, db, "test_auth_chpwd_weak", "Test@1234", "13800001007", 1)

	err := svc.ChangePassword(bgCtx,user.ID, "Test@1234", "weak")
	assert.Error(t, err)
	assert.Equal(t, 10003, err.(service.AppError).Code, "弱密码应返回 10003")
}
