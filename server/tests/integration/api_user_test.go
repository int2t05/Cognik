//go:build integration

// api_user_test.go — 用户管理接口集成测试（users.md 全覆盖）。
//
// 测试端点：
//   - GET    /api/v1/admin/users
//   - POST   /api/v1/admin/users
//   - GET    /api/v1/admin/users/:id
//   - PUT    /api/v1/admin/users/:id
//   - PATCH  /api/v1/admin/users/:id/freeze
//   - PATCH  /api/v1/admin/users/:id/unfreeze
package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── List ─────────────────────────────────────────────────

func TestAPI_User_List(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/users?page=1&page_size=10", nil))
	users := body["data"].([]interface{})
	assert.GreaterOrEqual(t, len(users), 3, "至少应有 3 个预置用户 (admin/reporter/operator)")
	u := users[0].(map[string]interface{})
	assert.NotEmpty(t, u["username"])
	assert.NotEmpty(t, u["status"])
	assert.NotNil(t, body["total"], "响应应含 total 分页字段")
}

func TestAPI_User_ListWithKeyword(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/users?page=1&page_size=10&keyword=admin", nil))
	users := body["data"].([]interface{})
	assert.GreaterOrEqual(t, len(users), 1, "keyword=admin 应至少找到 1 个用户")
}

// ── Create ───────────────────────────────────────────────

func TestAPI_User_CreateSuccess(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "new_user", "password": "NewUser@123", "real_name": "Test",
		"phone": "13800002001", "email": "test@opsmind.local", "role_ids": []int64{},
	})
	assertCode(t, resp, 0)

	var userID int64
	ts.DB.Raw("SELECT id FROM users WHERE username = 'new_user'").Scan(&userID)
	assert.NotZero(t, userID, "用户应被创建")
}

func TestAPI_User_CreateDuplicate(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "dup_user", "password": "Valid@1234", "real_name": "Dup1", "phone": "13800002002",
	})

	assertConflict(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "dup_user", "password": "Valid@1234", "real_name": "Dup2", "phone": "13800002003",
	}))
}

func TestAPI_User_CreateMissingFields(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 缺少 password
	resp := ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "x", "real_name": "X", "phone": "13800002004",
	})
	assertBadRequest(t, resp)
}

func TestAPI_User_CreateWeakPassword(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "weakpw", "password": "short", "real_name": "W", "phone": "13800002005",
	})
	assertBadRequest(t, resp)
}

// ── Detail ───────────────────────────────────────────────

func TestAPI_User_Detail(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "detail_me", "password": "Detail@123", "real_name": "Detail", "phone": "13800002006",
	})
	var userID int64
	ts.DB.Raw("SELECT id FROM users WHERE username = 'detail_me'").Scan(&userID)

	body := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%d", userID), nil))
	detail := body["data"].(map[string]interface{})
	assert.Equal(t, "detail_me", detail["username"])
	assert.Equal(t, float64(1), detail["status"])
	assert.NotNil(t, detail["roles"], "应含 roles 字段")
}

func TestAPI_User_DetailNotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertNotFound(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/users/99999", nil))
}

// ── Update ───────────────────────────────────────────────

func TestAPI_User_UpdateSuccess(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "update_me", "password": "Update@123", "real_name": "Old", "phone": "13800002007",
	})
	var userID int64
	ts.DB.Raw("SELECT id FROM users WHERE username = 'update_me'").Scan(&userID)

	assertCode(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", userID),
		map[string]interface{}{"real_name": "Updated", "phone": "13800002998", "email": "new@opsmind.local"}), 0)

	body := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%d", userID), nil))
	assert.Equal(t, "Updated", body["data"].(map[string]interface{})["real_name"])
}

func TestAPI_User_UpdateNotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertNotFound(t, ts.doAuth(t, http.MethodPut, "/api/v1/admin/users/99999",
		map[string]interface{}{"real_name": "Ghost", "phone": "13800000000"}))
}

func TestAPI_User_UpdateMissingRealName(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "upd_realname", "password": "UpdReal@123", "real_name": "Real", "phone": "13800002101",
	})
	var userID int64
	ts.DB.Raw("SELECT id FROM users WHERE username = 'upd_realname'").Scan(&userID)

	assertBadRequest(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", userID),
		map[string]interface{}{"real_name": "", "phone": "13800002998"}))
}

func TestAPI_User_UpdateMissingPhone(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "upd_phone", "password": "UpdPhone@1", "real_name": "Phone", "phone": "13800002102",
	})
	var userID int64
	ts.DB.Raw("SELECT id FROM users WHERE username = 'upd_phone'").Scan(&userID)

	assertBadRequest(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", userID),
		map[string]interface{}{"real_name": "Updated", "phone": ""}))
}

// ── Freeze ───────────────────────────────────────────────

func TestAPI_User_FreezeNotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertNotFound(t, ts.doAuth(t, http.MethodPatch, "/api/v1/admin/users/99999/freeze", nil))
}

func TestAPI_User_FreezeSuccess(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "freeze_me", "password": "Freeze@123", "real_name": "Fz", "phone": "13800002008",
	})
	var userID int64
	ts.DB.Raw("SELECT id FROM users WHERE username = 'freeze_me'").Scan(&userID)

	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/freeze", userID), nil), 0)
}

func TestAPI_User_FreezeSelf(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// admin 不能冻结自己
	assertBadRequest(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/freeze", ts.AdminID), nil))
}

func TestAPI_User_FreezeAlreadyFrozen(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "freeze2x", "password": "Freezx@123", "real_name": "Fz2", "phone": "13800002009",
	})
	var userID int64
	ts.DB.Raw("SELECT id FROM users WHERE username = 'freeze2x'").Scan(&userID)

	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/freeze", userID), nil), 0)
	// 重复冻结 → 10006
	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/freeze", userID), nil), 10006)
}

// ── Unfreeze ─────────────────────────────────────────────

func TestAPI_User_UnfreezeSuccess(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "unfreeze", "password": "Unfreeze@1", "real_name": "Uf", "phone": "13800002010",
	})
	var userID int64
	ts.DB.Raw("SELECT id FROM users WHERE username = 'unfreeze'").Scan(&userID)

	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/freeze", userID), nil), 0)
	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/unfreeze", userID), nil), 0)
}

func TestAPI_User_UnfreezeAlreadyActive(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 未冻结的用户恢复 → 10007
	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/unfreeze", ts.ReporterID), nil), 10007)
}

func TestAPI_User_UnfreezeNotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertNotFound(t, ts.doAuth(t, http.MethodPatch, "/api/v1/admin/users/99999/unfreeze", nil))
}

// ── Frozen login ─────────────────────────────────────────

func TestAPI_User_FrozenCannotLogin(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "frozen2", "password": "Frozen1@3", "real_name": "Fz", "phone": "13800002011",
	})
	var userID int64
	ts.DB.Raw("SELECT id FROM users WHERE username = 'frozen2'").Scan(&userID)
	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/freeze", userID), nil), 0)

	body := parseBody(t, ts.do(t, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": "frozen2", "password": "Frozen1@3"}, ""))
	assert.NotEqual(t, float64(0), body["code"], "冻结后应无法登录")
}
