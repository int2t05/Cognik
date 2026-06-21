/**
 * 角色管理 API 集成测试。
 *
 * 覆盖：列表/创建/详情/更新/删除 + 菜单列表。
 * 注意：创建角色返回 data:null → 通过列表搜索获取 ID。
 */
import { test, expect } from '@playwright/test';
import {
  API_URL,
  loginAsAdmin,
  authHeaders,
  assertSuccess,
  assertError,
  uniqueName,
} from './helpers';

test.describe('角色管理 API', () => {
  let token = '';

  test.beforeAll(async ({ request }) => {
    const auth = await loginAsAdmin(request);
    token = auth.accessToken;
  });

  test('获取角色列表', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/v1/admin/roles?page=1&page_size=10`, {
      headers: authHeaders(token),
    });
    const json = await assertSuccess(res);
    expect(Array.isArray(json.data)).toBe(true);
    expect(json.total).toBeGreaterThanOrEqual(4);
  });

  test('角色详情', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/v1/admin/roles/1`, {
      headers: authHeaders(token),
    });
    const json = await assertSuccess(res);
    expect(json.data.name).toBe('系统管理员');
    expect(json.data.permissions).toContain('user:manage');
  });

  test('不存在角色返回 404', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/v1/admin/roles/99999`, {
      headers: authHeaders(token),
    });
    await assertError(res, 10004, 404);
  });

  test.describe('CRUD', () => {
    let roleId = 0;
    let roleName = '';

    /** 按名称搜索角色，返回 ID */
    async function findRoleId(request: APIRequestContext, name: string): Promise<number> {
      const res = await request.get(
        `${API_URL}/api/v1/admin/roles?page=1&page_size=100&keyword=${encodeURIComponent(name)}`,
        { headers: authHeaders(token) },
      );
      const json = await res.json();
      if (json.code === 0 && json.data?.length > 0) {
        return json.data[0].id;
      }
      return 0;
    }

    test('创建角色', async ({ request }) => {
      roleName = uniqueName('测试角色');
      const res = await request.post(`${API_URL}/api/v1/admin/roles`, {
        data: { name: roleName, description: 'API 测试', permissions: ['dashboard:read', 'audit:read'] },
        headers: authHeaders(token),
      });
      await assertSuccess(res);
      // 后端返回 data:null → 通过搜索获取 ID
      roleId = await findRoleId(request, roleName);
    });

    test('重复角色名返回 409', async ({ request }) => {
      expect(roleName).toBeTruthy();
      if (!roleName) { test.skip(); return; }
      const res = await request.post(`${API_URL}/api/v1/admin/roles`, {
        data: { name: roleName, permissions: ['dashboard:read'] },
        headers: authHeaders(token),
      });
      await assertError(res, 10005, 409);
    });

    test('更新角色', async ({ request }) => {
      if (!roleId) { test.skip(); return; }
      const newName = uniqueName('更新角色');
      const res = await request.put(`${API_URL}/api/v1/admin/roles/${roleId}`, {
        data: { name: newName, description: '已更新', permissions: ['audit:read'] },
        headers: authHeaders(token),
      });
      await assertSuccess(res);

      const detail = await request.get(`${API_URL}/api/v1/admin/roles/${roleId}`, {
        headers: authHeaders(token),
      });
      const json = await assertSuccess(detail);
      expect(json.data.name).toBe(newName);
      expect(json.data.permissions).toEqual(['audit:read']);
    });

    test('删除无用户的角色', async ({ request }) => {
      // 创建新角色（通过名称搜索获取 ID）
      const tempName = uniqueName('临时删除');
      const createRes = await request.post(`${API_URL}/api/v1/admin/roles`, {
        data: { name: tempName, description: '待删除', permissions: ['dashboard:read'] },
        headers: authHeaders(token),
      });
      await assertSuccess(createRes);
      const deleteId = await findRoleId(request, tempName);
      if (!deleteId) { test.skip(); return; }

      const res = await request.delete(`${API_URL}/api/v1/admin/roles/${deleteId}`, {
        headers: authHeaders(token),
      });
      await assertSuccess(res);

      const detail = await request.get(`${API_URL}/api/v1/admin/roles/${deleteId}`, {
        headers: authHeaders(token),
      });
      await assertError(detail, 10004, 404);
    });

    test('删除内置角色返回 409', async ({ request }) => {
      const res = await request.delete(`${API_URL}/api/v1/admin/roles/1`, {
        headers: authHeaders(token),
      });
      await assertError(res, 10005, 409);
    });
  });

  test('获取菜单列表', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/v1/admin/menus`, {
      headers: authHeaders(token),
    });
    const json = await assertSuccess(res);
    expect(Array.isArray(json.data)).toBe(true);
    expect(json.data.length).toBeGreaterThanOrEqual(9);
  });
});
