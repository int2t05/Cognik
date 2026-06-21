//go:build integration

// api_auth_test.go — 认证接口集成测试（auth.md 全覆盖）。
//
// 测试端点：
//   - POST   /api/v1/auth/login
//   - POST   /api/v1/auth/refresh
//   - POST   /api/v1/auth/me/change-password
//   - POST   /api/v1/auth/me/logout
package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"opsmind/pkg/hash"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 辅助 ────────────────────────────────────────────────

// seedPlainUser 直接在 DB 创建用户（不通过 API），返回 username/token。
func seedPlainUser(t *testing.T, ts *apiTestServer, username, password, phone string, firstLogin bool) string {
	t.Helper()
	hashed, err := hash.HashPassword(password)
	require.NoError(t, err)
	ts.DB.Exec(`INSERT INTO users (username, password_hash, real_name, phone, status, first_login, created_at, updated_at)
		VALUES ($1, $2, 'test', $3, 1, $4, NOW(), NOW())`, username, hashed, phone, firstLogin)
	return ts.loginAs(t, username, password)
}

// ── Login ────────────────────────────────────────────────

func TestAPI_Auth_LoginSuccess(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "apitest_admin", "password": "Admin@123"}, ""))

	data := body["data"].(map[string]interface{})
	assert.NotEmpty(t, data["access_token"], "access_token 不应为空")
	assert.NotEmpty(t, data["refresh_token"], "refresh_token 不应为空")
	assert.NotNil(t, data["user"], "user 不应为空")
	assert.NotNil(t, data["roles"], "roles 不应为空")
	assert.NotNil(t, data["permissions"], "permissions 不应为空")
}

func TestAPI_Auth_LoginWrongPassword(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "apitest_admin", "password": "WrongPass@1"}, "")
	body := parseBody(t, resp)
	assert.NotEqual(t, float64(0), body["code"], "密码错误应返回非 0 code")
}

func TestAPI_Auth_LoginUserNotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "nobody", "password": "Whatever@1"}, "")
	body := parseBody(t, resp)
	// 用户名不存在与密码错误返回相同错误码，防止用户名枚举
	assert.NotEqual(t, float64(0), body["code"], "用户不存在应返回非 0 code")
}

func TestAPI_Auth_LoginMissingPassword(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "apitest_admin"}, "")
	assertHTTPStatus(t, resp, http.StatusBadRequest)
}

func TestAPI_Auth_LoginFrozenAccount(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	seedPlainUser(t, ts, "frozen_user", "Admin@123", "13800001001", false)
	ts.DB.Exec("UPDATE users SET status = 2 WHERE username = 'frozen_user'")

	resp := ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "frozen_user", "password": "Admin@123"}, "")
	body := parseBody(t, resp)
	assert.NotEqual(t, float64(0), body["code"], "冻结用户登录应被拒绝")
}

func TestAPI_Auth_LoginFirstLoginFlag(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	seedPlainUser(t, ts, "first_login", "Admin@123", "13800001002", true)

	body := assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "first_login", "password": "Admin@123"}, ""))

	user := body["data"].(map[string]interface{})["user"].(map[string]interface{})
	assert.False(t, user["first_login"].(bool), "首次登录后 first_login 应为 false")
}

// ── Refresh ──────────────────────────────────────────────

func TestAPI_Auth_RefreshSuccess(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "apitest_admin", "password": "Admin@123"}, ""))
	refresh := body["data"].(map[string]interface{})["refresh_token"].(string)

	body2 := assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/refresh",
		map[string]string{"refresh_token": refresh}, ""))
	data2 := body2["data"].(map[string]interface{})
	assert.NotEmpty(t, data2["access_token"], "刷新后应有新 access_token")
	assert.NotEmpty(t, data2["refresh_token"], "刷新后应有新 refresh_token")
}

func TestAPI_Auth_RefreshWithAccessToken(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "apitest_admin", "password": "Admin@123"}, ""))
	accessToken := body["data"].(map[string]interface{})["access_token"].(string)

	// 用 access_token 而非 refresh_token 调用 refresh → 10001
	resp := ts.do(t, http.MethodPost, "/api/v1/auth/refresh",
		map[string]string{"refresh_token": accessToken}, "")
	assertUnauthorized(t, resp)
}

func TestAPI_Auth_RefreshWithRevokedToken(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "apitest_admin", "password": "Admin@123"}, ""))
	data := body["data"].(map[string]interface{})
	access, refresh := data["access_token"].(string), data["refresh_token"].(string)

	// 登出使 refresh token 失效
	assertCode(t, ts.do(t, http.MethodPost, "/api/v1/auth/me/logout",
		map[string]string{"refresh_token": refresh}, access), 0)

	// 登出后 refresh 应失败
	assertUnauthorized(t, ts.do(t, http.MethodPost, "/api/v1/auth/refresh",
		map[string]string{"refresh_token": refresh}, ""))
}

func TestAPI_Auth_RefreshMissingToken(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.do(t, http.MethodPost, "/api/v1/auth/refresh",
		map[string]string{}, "")
	assertBadRequest(t, resp)
}

// ── Change Password ──────────────────────────────────────

func TestAPI_Auth_ChangePasswordSuccess(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	seedPlainUser(t, ts, "chpwd_ok", "OldPass@123", "13800001003", false)
	token := ts.loginAs(t, "chpwd_ok", "OldPass@123")

	assertCode(t, ts.do(t, http.MethodPost, "/api/v1/auth/me/change-password",
		map[string]string{"old_password": "OldPass@123", "new_password": "NewPass@456"}, token), 0)

	// 新密码可登录
	assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "chpwd_ok", "password": "NewPass@456"}, ""))

	// 旧密码失效
	body := parseBody(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "chpwd_ok", "password": "OldPass@123"}, ""))
	assert.NotEqual(t, float64(0), body["code"], "旧密码应失效")
}

func TestAPI_Auth_ChangePasswordWrongOld(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	token := seedPlainUser(t, ts, "chpwd_wrong", "Admin@123", "13800001004", false)

	resp := ts.do(t, http.MethodPost, "/api/v1/auth/me/change-password",
		map[string]string{"old_password": "WrongOld@1", "new_password": "NewPass@456"}, token)
	assertBadRequest(t, resp)
}

func TestAPI_Auth_ChangePasswordWeak(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	token := seedPlainUser(t, ts, "chpwd_weak", "Admin@123", "13800001005", false)

	resp := ts.do(t, http.MethodPost, "/api/v1/auth/me/change-password",
		map[string]string{"old_password": "Admin@123", "new_password": "short"}, token)
	assertBadRequest(t, resp)
}

func TestAPI_Auth_ChangePasswordUnauthenticated(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.do(t, http.MethodPost, "/api/v1/auth/me/change-password",
		map[string]string{"old_password": "x", "new_password": "Valid@123"}, "")
	assertUnauthorized(t, resp)
}

// ── Logout ───────────────────────────────────────────────

func TestAPI_Auth_LogoutSuccess(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "apitest_admin", "password": "Admin@123"}, ""))
	data := body["data"].(map[string]interface{})
	token, refresh := data["access_token"].(string), data["refresh_token"].(string)

	assertCode(t, ts.do(t, http.MethodPost, "/api/v1/auth/me/logout",
		map[string]string{"refresh_token": refresh}, token), 0)

	// 登出后 refresh 失败
	assertUnauthorized(t, ts.do(t, http.MethodPost, "/api/v1/auth/refresh",
		map[string]string{"refresh_token": refresh}, ""))
}

func TestAPI_Auth_LogoutMissingToken(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "apitest_admin", "password": "Admin@123"}, ""))
	token := body["data"].(map[string]interface{})["access_token"].(string)

	resp := ts.do(t, http.MethodPost, "/api/v1/auth/me/logout",
		map[string]string{}, token)
	assertBadRequest(t, resp)
}

// ── Rate Limit ─────────────────────────────────────────────

func TestAPI_Auth_LoginRateLimit(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	for i := 0; i < 7; i++ {
		resp := ts.do(t, http.MethodPost, "/api/v1/auth/login",
			map[string]string{"username": "apitest_admin", "password": "WrongPass@1"}, "")
		body := parseBody(t, resp)
		code := body["code"].(float64)
		assert.NotEqual(t, float64(0), code, "第 %d 次登录应失败", i+1)
		assert.NotEqual(t, float64(99999), code, "第 %d 次登录不应服务器错误", i+1)
	}
}

// ── Refresh Frozen User ────────────────────────────────────

func TestAPI_Auth_RefreshFrozenUser(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 直接创建用户（不通过 API）
	pwd := "Freeze@123"
	hashed, err := hash.HashPassword(pwd)
	require.NoError(t, err)
	ts.DB.Exec(`INSERT INTO users (username, password_hash, real_name, phone, status, first_login, created_at, updated_at)
		VALUES ('refresh_frozen', $1, 'Test', '13800003001', 1, false, NOW(), NOW())`, hashed)

	var userID int64
	ts.DB.Raw("SELECT id FROM users WHERE username = 'refresh_frozen'").Scan(&userID)
	require.NotZero(t, userID)

	// 登录获取 tokens
	body := assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "refresh_frozen", "password": pwd}, ""))
	data := body["data"].(map[string]interface{})
	refreshToken := data["refresh_token"].(string)

	// 冻结用户
	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/freeze", userID), nil), 0)

	// 刷新应被拒绝（用户已冻结）
	assertForbidden(t, ts.do(t, http.MethodPost, "/api/v1/auth/refresh",
		map[string]string{"refresh_token": refreshToken}, ""))
}

// ── Logout Idempotent ──────────────────────────────────────

func TestAPI_Auth_LogoutIdempotent(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "apitest_admin", "password": "Admin@123"}, ""))
	data := body["data"].(map[string]interface{})
	token := data["access_token"].(string)
	refresh := data["refresh_token"].(string)

	// 第一次登出
	assertCode(t, ts.do(t, http.MethodPost, "/api/v1/auth/me/logout",
		map[string]string{"refresh_token": refresh}, token), 0)

	// 第二次登出同样成功（幂等）
	assertCode(t, ts.do(t, http.MethodPost, "/api/v1/auth/me/logout",
		map[string]string{"refresh_token": refresh}, token), 0)
}
