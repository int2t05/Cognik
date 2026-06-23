//go:build integration

// Package handler_test 验证认证 Handler 的 HTTP 接口行为。
//
// 测试覆盖 PLAN.md T11 定义的场景：
// POST /login 正常返回、参数缺失返回 400。
package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"opsmind/internal/config"
	"opsmind/internal/database"
	"opsmind/internal/handler"
	"opsmind/internal/model"
	"opsmind/internal/repository"
	"opsmind/internal/service"
	"opsmind/pkg/hash"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// testJWTConfig 返回测试用 JWT 配置，与 config.yaml 默认值一致。
func testJWTConfig() config.JWTConfig {
	return config.JWTConfig{
		Secret:        "test_secret_key_2024",
		AccessExpire:  2 * time.Hour,
		RefreshExpire: 168 * time.Hour,
	}
}

// setupTestRouter 创建测试用 Gin 引擎，绑定认证路由。
func setupTestRouter(authHandler *handler.AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// 公开路由
	auth := r.Group("/api/v1/auth")
	auth.POST("/login", authHandler.Login)
	auth.POST("/refresh", authHandler.Refresh)
	// 需要 JWT 的路由（对应实际 router 中 /api/v1/auth/me）
	authMe := r.Group("/api/v1/auth/me")
	authMe.POST("/change-password", authHandler.ChangePassword)
	authMe.POST("/logout", authHandler.Logout)
	return r
}

// setupHandlerTestDB 初始化测试数据库。
func setupHandlerTestDB(t *testing.T) *gorm.DB {
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

// setupAuthHandler 创建测试用 Gin 引擎和 Handler。
func setupAuthHandler(t *testing.T, db *gorm.DB) *handler.AuthHandler {
	t.Helper()

	userRepo := repository.NewUserRepo(db)
	menuRepo := repository.NewMenuRepo(db)
	authService := service.NewAuthService(userRepo, menuRepo, db, testJWTConfig())
	return handler.NewAuthHandler(authService)
}

// seedHandlerUser 创建测试用户并确保唯一手机号。
func seedHandlerUser(t *testing.T, db *gorm.DB, username, password, phone string, status int16) *model.User {
	t.Helper()

	hashed, err := hash.HashPassword(password)
	require.NoError(t, err)

	// 清理同名用户和同手机号残留
	db.Where("username = ?", username).Delete(&model.User{})
	db.Where("phone = ?", phone).Delete(&model.User{})

	user := &model.User{
		Username:     username,
		PasswordHash: hashed,
		RealName:     "测试用户",
		Phone:        phone,
		Status:       status,
		FirstLogin:   true,
	}

	require.NoError(t, db.Create(user).Error)
	return user
}

// TestAuthHandler_Login_Success 验证 POST /login 正常返回。
func TestAuthHandler_Login_Success(t *testing.T) {
	db := setupHandlerTestDB(t)
	authHandler := setupAuthHandler(t, db)
	seedHandlerUser(t, db, "test_handler_login", "Test@1234", "13800002001", 1)

	r := setupTestRouter(authHandler)

	body, _ := json.Marshal(map[string]string{
		"username": "test_handler_login",
		"password": "Test@1234",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "登录成功应返回 200")

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(0), resp["code"], "业务码应为 0")

	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok, "data 应为对象")
	assert.NotEmpty(t, data["access_token"], "应包含 access_token")
	assert.NotEmpty(t, data["refresh_token"], "应包含 refresh_token")
}

// TestAuthHandler_Login_MissingParams 验证参数缺失返回 400。
func TestAuthHandler_Login_MissingParams(t *testing.T) {
	db := setupHandlerTestDB(t)
	authHandler := setupAuthHandler(t, db)

	r := setupTestRouter(authHandler)

	// 缺少 password
	body, _ := json.Marshal(map[string]string{
		"username": "test_handler_login",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "参数缺失应返回 400")
}

// TestAuthHandler_Login_WrongPassword 验证密码错误返回 400。
func TestAuthHandler_Login_WrongPassword(t *testing.T) {
	db := setupHandlerTestDB(t)
	authHandler := setupAuthHandler(t, db)
	seedHandlerUser(t, db, "test_handler_wrong", "Test@1234", "13800002002", 1)

	r := setupTestRouter(authHandler)

	body, _ := json.Marshal(map[string]string{
		"username": "test_handler_wrong",
		"password": "WrongPass@1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, "密码错误应返回 400")

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(10003), resp["code"])
}

// TestAuthHandler_Logout 验证 POST /logout 返回成功。
//
// logout 路由注册在 /api/v1/auth/me 下，需要 JWT 认证。
func TestAuthHandler_Logout(t *testing.T) {
	db := setupHandlerTestDB(t)
	authHandler := setupAuthHandler(t, db)
	seedHandlerUser(t, db, "test_handler_logout", "Test@1234", "13800002002", 1)

	r := setupTestRouter(authHandler)

	// 先登录获取 token
	loginBody, _ := json.Marshal(map[string]string{
		"username": "test_handler_logout", "password": "Test@1234",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	r.ServeHTTP(loginW, loginReq)
	require.Equal(t, http.StatusOK, loginW.Code)

	var loginResp map[string]interface{}
	json.Unmarshal(loginW.Body.Bytes(), &loginResp)
	// 使用 refresh_token 请求 logout
	refreshToken := loginResp["data"].(map[string]interface{})["refresh_token"].(string)
	logoutBody, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/me/logout", bytes.NewReader(logoutBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "logout 应返回 200")
}
