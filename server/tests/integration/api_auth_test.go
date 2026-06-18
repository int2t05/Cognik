//go:build integration

package integration_test

import (
	"net/http"
	"testing"

	"opsmind/pkg/hash"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedUser(t *testing.T, ts *apiTestServer, username, password, phone string, firstLogin bool) {
	t.Helper()
	hashed, err := hash.HashPassword(password)
	require.NoError(t, err)
	ts.DB.Exec(`INSERT INTO users (username, password_hash, real_name, phone, status, first_login, created_at, updated_at)
		VALUES ($1, $2, 'test', $3, 1, $4, NOW(), NOW())`, username, hashed, phone, firstLogin)
}

func TestAPI_Auth_FullLifecycle(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	username, oldPwd, newPwd := "auth_life", "OldPass@123", "NewPass@456"
	seedUser(t, ts, username, oldPwd, "13800001001", false)

	// 登录
	body := assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": username, "password": oldPwd}, ""))
	data := body["data"].(map[string]interface{})
	assert.NotEmpty(t, data["access_token"])
	assert.NotEmpty(t, data["refresh_token"])

	// 刷新
	assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/refresh",
		map[string]string{"refresh_token": data["refresh_token"].(string)}, ""))

	// 改密
	assertCode(t, ts.do(t, http.MethodPost, "/api/v1/auth/me/change-password",
		map[string]string{"old_password": oldPwd, "new_password": newPwd}, data["access_token"].(string)), 0)

	// 新密码登录
	assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": username, "password": newPwd}, ""))

	// 旧密码失效
	body5 := parseBody(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": username, "password": oldPwd}, ""))
	assert.NotEqual(t, float64(0), body5["code"])
}

func TestAPI_Auth_FirstLoginFlag(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	seedUser(t, ts, "auth_first", "Admin@123", "13800001002", true)
	body := assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "auth_first", "password": "Admin@123"}, ""))
	user := body["data"].(map[string]interface{})["user"].(map[string]interface{})
	// 首次登录后 first_login 应由服务端自动置为 false
	assert.False(t, user["first_login"].(bool), "首次登录后 first_login 应为 false")
	assert.NotEmpty(t, body["data"].(map[string]interface{})["access_token"])
}

func TestAPI_Auth_LoginFailures(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	seedUser(t, ts, "auth_fail", "Admin@123", "13800001003", false)

	assert.NotEqual(t, float64(0), parseBody(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "auth_fail", "password": "Wrong@1"}, ""))["code"])
	assert.NotEqual(t, float64(0), parseBody(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "nobody", "password": "Whatever@1"}, ""))["code"])
	assert.Equal(t, http.StatusBadRequest, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "auth_fail"}, "").StatusCode)
}

func TestAPI_Auth_ChangePasswordValidation(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	seedUser(t, ts, "auth_chpwd", "Admin@123", "13800001004", false)
	loginBody := assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "auth_chpwd", "password": "Admin@123"}, ""))
	token := loginBody["data"].(map[string]interface{})["access_token"].(string)

	assert.NotEqual(t, float64(0), parseBody(t, ts.do(t, http.MethodPost, "/api/v1/auth/me/change-password",
		map[string]string{"old_password": "Admin@123", "new_password": "weak"}, token))["code"])
	assert.NotEqual(t, float64(0), parseBody(t, ts.do(t, http.MethodPost, "/api/v1/auth/me/change-password",
		map[string]string{"old_password": "Wrong@123", "new_password": "NewValid@123"}, token))["code"])
}

func TestAPI_Auth_Logout(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "apitest_admin", "password": "Admin@123"}, ""))
	data := body["data"].(map[string]interface{})
	token, refresh := data["access_token"].(string), data["refresh_token"].(string)

	assertCode(t, ts.do(t, http.MethodPost, "/api/v1/auth/me/logout",
		map[string]string{"refresh_token": refresh}, token), 0)

	assert.NotEqual(t, float64(0), parseBody(t, ts.do(t, http.MethodPost, "/api/v1/auth/refresh",
		map[string]string{"refresh_token": refresh}, ""))["code"])
}

func TestAPI_Auth_FrozenAccount(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	seedUser(t, ts, "auth_frozen", "Admin@123", "13800001005", false)
	ts.DB.Exec("UPDATE users SET status = 2 WHERE username = 'auth_frozen'")

	assert.NotEqual(t, float64(0), parseBody(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "auth_frozen", "password": "Admin@123"}, ""))["code"])
}
