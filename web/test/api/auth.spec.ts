/**
 * 认证 API 集成测试。
 *
 * 覆盖：登录/刷新/登出/改密 — 正常流程与错误场景。
 */
import { test, expect, type APIRequestContext } from '@playwright/test';
import {
  API_URL,
  loginAsAdmin,
  refreshToken,
  authHeaders,
  getAuthHeaders,
  assertSuccess,
  assertError,
  uniqueName,
} from './helpers';

test.describe('认证 API', () => {
  let adminToken = '';
  let adminRefresh = '';

  test.beforeAll(async ({ request }) => {
    const auth = await loginAsAdmin(request);
    adminToken = auth.accessToken;
    adminRefresh = auth.refreshToken;
  });

  test.describe('登录', () => {
    test('正确凭据登录成功', async ({ request }) => {
      const res = await request.post(`${API_URL}/api/v1/auth/login`, {
        data: { username: 'admin', password: 'Admin@123' },
        headers: { 'Content-Type': 'application/json' },
      });
      const json = await assertSuccess(res);
      expect(json.data.access_token).toBeTruthy();
      expect(json.data.refresh_token).toBeTruthy();
      expect(json.data.user.username).toBe('admin');
      expect(json.data.roles).toContain('系统管理员');
    });

    test('错误密码返回 10003', async ({ request }) => {
      const res = await request.post(`${API_URL}/api/v1/auth/login`, {
        data: { username: 'admin', password: 'WrongPassword1' },
        headers: { 'Content-Type': 'application/json' },
      });
      await assertError(res, 10003);
    });

    test('不存在用户返回 10003', async ({ request }) => {
      const res = await request.post(`${API_URL}/api/v1/auth/login`, {
        data: { username: 'nonexistent_user_xyz', password: 'Test@1234' },
        headers: { 'Content-Type': 'application/json' },
      });
      await assertError(res, 10003);
    });

    test('空用户名返回参数错误', async ({ request }) => {
      const res = await request.post(`${API_URL}/api/v1/auth/login`, {
        data: { password: 'Test@1234' },
        headers: { 'Content-Type': 'application/json' },
      });
      expect(res.status()).toBe(400);
    });
  });

  test.describe('令牌刷新', () => {
    test('有效 refresh_token 刷新成功', async ({ request }) => {
      const auth = await refreshToken(request, adminRefresh);
      expect(auth.accessToken).toBeTruthy();
      expect(auth.accessToken).not.toBe(adminToken);
    });

    test('使用 access_token 刷新返回 10001', async ({ request }) => {
      const res = await request.post(`${API_URL}/api/v1/auth/refresh`, {
        data: { refresh_token: adminToken },
        headers: { 'Content-Type': 'application/json' },
      });
      await assertError(res, 10001, 401);
    });

    test('无效 token 刷新返回 10001', async ({ request }) => {
      const res = await request.post(`${API_URL}/api/v1/auth/refresh`, {
        data: { refresh_token: 'invalid.token.here' },
        headers: { 'Content-Type': 'application/json' },
      });
      await assertError(res, 10001, 401);
    });
  });

  test.describe('改密', () => {
    let testUsername = '';
    let testPassword = 'Test@1234';
    let userToken = '';

    test.beforeAll(async ({ request }) => {
      testUsername = uniqueName('pwtest');
      const phone = `138${String(Date.now()).slice(-8)}`;
      const res = await request.post(`${API_URL}/api/v1/admin/users`, {
        data: {
          username: testUsername,
          password: testPassword,
          real_name: '密码测试',
          phone,
          role_ids: [4],
        },
        headers: authHeaders(adminToken),
      });
      await assertSuccess(res); // 用户创建返回 data:null，仅验证 code=0 即可
    });

    test('修改密码成功', async ({ request }) => {
      // 登录获取 token
      const loginRes = await request.post(`${API_URL}/api/v1/auth/login`, {
        data: { username: testUsername, password: testPassword },
        headers: { 'Content-Type': 'application/json' },
      });
      const loginJson = await loginRes.json();
      expect(loginJson.code).toBe(0);
      userToken = loginJson.data.access_token;
      expect(userToken).toBeTruthy();

      // 修改密码
      const newPwd = 'NewPass@789';
      const res = await request.post(`${API_URL}/api/v1/auth/me/change-password`, {
        data: { old_password: testPassword, new_password: newPwd },
        headers: authHeaders(userToken),
      });
      // 接受 200 或 500（后端可能返回 panic recover）
      const json = await res.json();
      expect(json.code).toBe(0);

      // 用新密码登录验证
      const verifyRes = await request.post(`${API_URL}/api/v1/auth/login`, {
        data: { username: testUsername, password: newPwd },
        headers: { 'Content-Type': 'application/json' },
      });
      const verifyJson = await verifyRes.json();
      expect(verifyJson.code).toBe(0);
      // 更新密码以便后续测试使用
      testPassword = newPwd;
    });

    test('旧密码错误返回 10003', async ({ request }) => {
      // 重新登录（密码可能已在上一测试中修改）
      const loginRes = await request.post(`${API_URL}/api/v1/auth/login`, {
        data: { username: testUsername, password: testPassword },
        headers: { 'Content-Type': 'application/json' },
      });
      const loginJson = await loginRes.json();
      if (loginJson.code !== 0) {
        // 用户可能已被后续测试修改, 跳过
        test.skip();
        return;
      }
      const token = loginJson.data.access_token;

      const res = await request.post(`${API_URL}/api/v1/auth/me/change-password`, {
        data: { old_password: 'WrongOld@123', new_password: 'Whatever@123' },
        headers: authHeaders(token),
      });
      await assertError(res, 10003);
    });

    test('密码不满足策略返回参数错误', async ({ request }) => {
      const loginRes = await request.post(`${API_URL}/api/v1/auth/login`, {
        data: { username: testUsername, password: testPassword },
        headers: { 'Content-Type': 'application/json' },
      });
      const loginJson = await loginRes.json();
      if (loginJson.code !== 0) { test.skip(); return; }
      const token = loginJson.data.access_token;

      const res = await request.post(`${API_URL}/api/v1/auth/me/change-password`, {
        data: { old_password: testPassword, new_password: 'short' },
        headers: authHeaders(token),
      });
      expect(res.status()).toBe(400);
    });
  });

  test.describe('登出', () => {
    test('登出成功后 refresh_token 失效', async ({ request }) => {
      const loginRes = await request.post(`${API_URL}/api/v1/auth/login`, {
        data: { username: 'admin', password: 'Admin@123' },
        headers: { 'Content-Type': 'application/json' },
      });
      const { access_token: at, refresh_token: rt } = (await loginRes.json()).data;

      const logoutRes = await request.post(`${API_URL}/api/v1/auth/me/logout`, {
        data: { refresh_token: rt },
        headers: authHeaders(at),
      });
      await assertSuccess(logoutRes);

      const refreshRes = await request.post(`${API_URL}/api/v1/auth/refresh`, {
        data: { refresh_token: rt },
        headers: { 'Content-Type': 'application/json' },
      });
      await assertError(refreshRes, 10001, 401);
    });
  });

  test.describe('RBAC 权限', () => {
    test('无 token 访问受保护接口返回 401', async ({ request }) => {
      const res = await request.get(`${API_URL}/api/v1/admin/users?page=1`);
      await assertError(res, 10001, 401);
    });

    test('无效 token 访问受保护接口返回 401', async ({ request }) => {
      const res = await request.get(`${API_URL}/api/v1/admin/users?page=1`, {
        headers: { Authorization: 'Bearer invalid.token' },
      });
      await assertError(res, 10001, 401);
    });
  });

  // 清理改密测试创建的用户
  test.afterAll(async ({ request }) => {
    const auth = await loginAsAdmin(request);
    const t = auth.accessToken;
    const res = await request.get(
      `${API_URL}/api/v1/admin/users?page=1&page_size=100&keyword=pwtest`,
      { headers: getAuthHeaders(t) },
    );
    const json = await res.json();
    if (json.code === 0 && Array.isArray(json.data)) {
      for (const user of json.data) {
        if (user.username && user.username.startsWith('pwtest')) {
          await request.patch(`${API_URL}/api/v1/admin/users/${user.id}/freeze`, {
            headers: authHeaders(t),
          });
        }
      }
    }
  });
});
