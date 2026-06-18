//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPI_User_Lifecycle(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	assertCode(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "new_user", "password": "NewUser@123", "real_name": "Test", "phone": "13800002001",
	}), 0)
	var userID int64
	ts.DB.Raw("SELECT id FROM users WHERE username = 'new_user'").Scan(&userID)
	assert.NotZero(t, userID)

	assert.GreaterOrEqual(t, len(assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/users?page=1&page_size=10", nil))["data"].([]interface{})), 2)
	assert.Equal(t, "new_user", assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%d", userID), nil))["data"].(map[string]interface{})["username"])

	assertCode(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", userID),
		map[string]interface{}{"real_name": "Updated", "phone": "13800002999"}), 0)
	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/freeze", userID), nil), 0)
	assertCode(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/unfreeze", userID), nil), 0)
}

func TestAPI_User_Validation(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "dup", "password": "Valid@1234", "real_name": "Dup", "phone": "13800002002",
	})

	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "dup", "password": "Valid@1234", "real_name": "Dup2", "phone": "13800002003",
	}))["code"])
	assert.Equal(t, http.StatusBadRequest, ts.doAuth(t, http.MethodPost, "/api/v1/admin/users", map[string]interface{}{
		"username": "weak", "password": "short", "real_name": "W", "phone": "13800002004",
	}).StatusCode)
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/users/99999", nil))["code"])
	assert.NotEqual(t, float64(0), parseBody(t, ts.doAuth(t, http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d/freeze", ts.AdminID), nil))["code"])
}

func TestAPI_Role_Lifecycle(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()

	roleID := ts.seedRole(t, "test_role", []string{"ticket:read"})

	assert.GreaterOrEqual(t, len(assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/roles?page=1&page_size=10", nil))["data"].([]interface{})), 1)
	assert.Equal(t, "test_role", assertOK(t, ts.doAuth(t, http.MethodGet, fmt.Sprintf("/api/v1/admin/roles/%d", roleID), nil))["data"].(map[string]interface{})["name"])

	assertCode(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/roles/%d", roleID),
		map[string]interface{}{"name": "test_role_v2", "description": "upd", "permissions": []string{"ticket:read"}}), 0)
	assertCode(t, ts.doAuth(t, http.MethodDelete, fmt.Sprintf("/api/v1/admin/roles/%d", roleID), nil), 0)
}

func TestAPI_Menu_List(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()
	assertOK(t, ts.doAuth(t, http.MethodGet, "/api/v1/admin/menus", nil))
}

func TestAPI_RoleMenu_Update(t *testing.T) {
	ts := startAPITestServer(t)
	defer ts.close()
	roleID := ts.seedRole(t, "menu_role", []string{"user:manage"})
	assertCode(t, ts.doAuth(t, http.MethodPut, fmt.Sprintf("/api/v1/admin/roles/%d/menus", roleID),
		map[string]interface{}{"menu_ids": []int64{}}), 0)
}
