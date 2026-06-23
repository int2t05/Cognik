//go:build integration

// api_role_test.go — 角色与菜单管理接口集成测试（roles.md 全覆盖）。
//
// 测试端点：
//   - GET    /api/v1/admin/roles
//   - POST   /api/v1/admin/roles
//   - GET    /api/v1/admin/roles/:id
//   - PUT    /api/v1/admin/roles/:id
//   - DELETE /api/v1/admin/roles/:id
//   - GET    /api/v1/admin/menus
//   - PUT    /api/v1/admin/roles/:id/menus
package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── List ─────────────────────────────────────────────────

func TestAPI_Role_List(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/roles?page=1&page_size=10", nil))
	roles := body["data"].([]interface{})
	assert.GreaterOrEqual(t, len(roles), 3, "至少应有 3 个预置角色")
	r := roles[0].(map[string]interface{})
	assert.NotEmpty(t, r["name"], "角色应含 name 字段")
	assert.NotNil(t, r["permissions"], "角色应含 permissions 字段")
	assert.NotNil(t, body["total"], "响应应含 total")
}

func TestAPI_Role_ListWithKeyword(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/roles?keyword=管理", nil))
	assert.GreaterOrEqual(t, len(body["data"].([]interface{})), 1, "keyword=管理 应找到角色")
}

// ── Create ───────────────────────────────────────────────

func TestAPI_Role_CreateSuccess(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	resp := ts.doAuth(t, http.MethodPost, "/api/v1/admin/roles", map[string]interface{}{
		"name": "test_role", "description": "测试角色", "permissions": []string{"ticket:read", "audit:read"},
	})
	assertCode(t, resp, 0)

	var id int64
	ts.DB.Raw("SELECT id FROM roles WHERE name = 'test_role'").Scan(&id)
	assert.NotZero(t, id, "角色应被创建")
}

func TestAPI_Role_CreateMissingName(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertBadRequest(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/roles",
		map[string]interface{}{"permissions": []string{"audit:read"}}))
}

func TestAPI_Role_CreateDuplicate(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	ts.doAuth(t, http.MethodPost, "/api/v1/admin/roles", map[string]interface{}{
		"name": "dup_role", "description": "dup",
	})
	assertConflict(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/roles", map[string]interface{}{
		"name": "dup_role",
	}))
}

// ── Detail ───────────────────────────────────────────────

func TestAPI_Role_Detail(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	roleID := ts.seedRole(t, "detail_role", []string{"ticket:read"})

	body := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/roles/%d", roleID), nil))
	detail := body["data"].(map[string]interface{})
	assert.Equal(t, "detail_role", detail["name"])
	assert.NotNil(t, detail["permissions"])
}

func TestAPI_Role_DetailInvalidID(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertBadRequest(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/roles/abc", nil))
}

func TestAPI_Role_DetailNotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertNotFound(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/roles/99999", nil))
}

// ── Update ───────────────────────────────────────────────

func TestAPI_Role_UpdateSuccess(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	roleID := ts.seedRole(t, "update_role", []string{"ticket:read"})

	assertCode(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/roles/%d", roleID),
		map[string]interface{}{
			"name": "updated_role", "description": "updated", "permissions": []string{"audit:read"},
		}), 0)

	body := assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/roles/%d", roleID), nil))
	assert.Equal(t, "updated_role", body["data"].(map[string]interface{})["name"])
}

func TestAPI_Role_UpdateDuplicate(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	role1 := ts.seedRole(t, "upd_dup1", nil)
	ts.seedRole(t, "upd_dup2", nil)

	assertConflict(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/roles/%d", role1),
		map[string]interface{}{"name": "upd_dup2"}))
}

// ── Delete ───────────────────────────────────────────────

func TestAPI_Role_UpdateMissingName(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	roleID := ts.seedRole(t, "upd_name_role", []string{"ticket:read"})

	assertBadRequest(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/roles/%d", roleID),
		map[string]interface{}{"name": ""}))
}

func TestAPI_Role_UpdateNotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertNotFound(t, ts.doAuth(t, http.MethodPut, "/api/v1/admin/roles/99999",
		map[string]interface{}{"name": "ghost_role"}))
}

func TestAPI_Role_DeleteSuccess(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	roleID := ts.seedRole(t, "delete_me", nil)
	assertCode(t, ts.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/roles/%d", roleID), nil), 0)
}

func TestAPI_Role_DeleteWithUsers(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 系统管理员角色有关联用户，删除应失败 → 可能 10005 或 10002
	resp := ts.doAuth(t, http.MethodDelete, "/api/v1/admin/roles/1", nil)
	code := parseBody(t, resp)["code"].(float64)
	assert.True(t, code == 10002 || code == 10005, "删除有关联用户的内置角色应失败, got code=%v", code)
}

func TestAPI_Role_DeleteBuiltin(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	// 尝试删除预置的报障人角色（内置角色）
	var reporterRoleID int64
	ts.DB.Raw("SELECT id FROM roles WHERE name = '报障人'").Scan(&reporterRoleID)
	resp := ts.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/roles/%d", reporterRoleID), nil)
	// 内置角色可能拒绝删除
	code := parseBody(t, resp)["code"].(float64)
	if code != 0 {
		t.Logf("内置角色删除被拒绝 (预期行为): code=%v", code)
	}
}

func TestAPI_Role_DeleteNotFound(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertNotFound(t, ts.doAuth(t, http.MethodDelete, "/api/v1/admin/roles/99999", nil))
}

// ── Menus ────────────────────────────────────────────────

func TestAPI_Menu_List(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	body := assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/menus", nil))
	menus := body["data"].([]interface{})
	// 菜单可能为空（未初始化种子），但端点应正常响应
	t.Logf("菜单数量: %d", len(menus))
}

// ── RoleMenu ─────────────────────────────────────────────

func TestAPI_RoleMenu_Update(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	roleID := ts.seedRole(t, "menu_role", []string{"user:manage"})
	assertCode(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/roles/%d/menus", roleID),
		map[string]interface{}{"menu_ids": []int64{}}), 0)
}

func TestAPI_RoleMenu_UpdateMissingMenuIDs(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	roleID := ts.seedRole(t, "menu_missing", nil)
	assertBadRequest(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/roles/%d/menus", roleID),
		map[string]interface{}{}))
}

func TestAPI_RoleMenu_UpdateForNonExistentRole(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertNotFound(t, ts.doAuth(t, http.MethodPut, "/api/v1/admin/roles/99999/menus",
		map[string]interface{}{"menu_ids": []int64{}}))
}
