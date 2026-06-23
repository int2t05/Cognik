/**
 * 用户管理 API 集成测试。
 *
 * 覆盖：列表/创建/详情/更新/冻结/恢复 + 参数校验。
 * 用户创建返回 data:null → 通过搜索获取 ID。
 */
import { test, expect, type APIRequestContext } from '@playwright/test';
import {
  API_URL,
  loginAsAdmin,
  authHeaders,
  getAuthHeaders,
  assertSuccess,
  assertError,
  uniqueName,
  uniquePhone,
} from './helpers';

test.describe('用户管理 API', () => {
  let token = '';
  const createdUsernames: string[] = [];

  test.beforeAll(async ({ request }) => {
    const auth = await loginAsAdmin(request);
    token = auth.accessToken;
  });

  test.afterAll(async ({ request }) => {
    // 清理：冻结创建的测试用户
    for (const uname of createdUsernames) {
      const list = await request.get(
        `${API_URL}/api/v1/admin/users?page=1&page_size=100&keyword=${uname}`,
        { headers: getAuthHeaders(token) },
      );
      const json = await list.json();
      if (json.code === 0 && json.data?.length > 0) {
        for (const u of json.data) {
          try {
            await request.patch(`${API_URL}/api/v1/admin/users/${u.id}/freeze`, {
              headers: authHeaders(token),
            });
          } catch { /* 忽略 */ }
        }
      }
    }
  });

  /** 创建用户并返回其详情（因创建接口返回 data:null，需查询获取 ID） */
  async function createUser(
    request: APIRequestContext,
    payload: Record<string, unknown>,
  ) {
    createdUsernames.push(payload.username as string);
    const res = await request.post(`${API_URL}/api/v1/admin/users`, {
      data: payload,
      headers: authHeaders(token),
    });
    await assertSuccess(res);

    // 通过搜索获取创建的用户
    const list = await request.get(
      `${API_URL}/api/v1/admin/users?page=1&page_size=10&keyword=${payload.username}`,
      { headers: getAuthHeaders(token) },
    );
    const listJson = await assertSuccess(list);
    expect(listJson.data.length).toBe(1);
    return listJson.data[0];
  }

  test.describe('列表', () => {
    test('获取用户列表（默认分页）', async ({ request }) => {
      const res = await request.get(`${API_URL}/api/v1/admin/users?page=1&page_size=10`, {
        headers: getAuthHeaders(token),
      });
      const json = await assertSuccess(res);
      expect(Array.isArray(json.data)).toBe(true);
      expect(json.total).toBeGreaterThan(0);
    });

    test('按关键词搜索', async ({ request }) => {
      const res = await request.get(
        `${API_URL}/api/v1/admin/users?page=1&page_size=10&keyword=admin`,
        { headers: getAuthHeaders(token) },
      );
      const json = await assertSuccess(res);
      expect(json.data.length).toBeGreaterThan(0);
      expect(json.data.some((u: { username: string }) => u.username === 'admin')).toBe(true);
    });
  });

  test.describe('创建', () => {
    test('创建用户成功', async ({ request }) => {
      const user = await createUser(request, {
        username: uniqueName('newuser'),
        password: 'NewUser@123',
        real_name: '测试用户',
        phone: uniquePhone(),
        role_ids: [4],
      });
      expect(user.username).toContain('newuser');
      expect(user.status).toBe(1);
    });

    test('缺少必填字段返回 400', async ({ request }) => {
      const res = await request.post(`${API_URL}/api/v1/admin/users`, {
        data: { username: 'baduser' },
        headers: authHeaders(token),
      });
      expect(res.status()).toBe(400);
    });

    test('密码不满足策略返回 400', async ({ request }) => {
      const res = await request.post(`${API_URL}/api/v1/admin/users`, {
        data: {
          username: uniqueName('shortpw'),
          password: 'short',
          real_name: '测试',
          phone: uniquePhone(),
        },
        headers: authHeaders(token),
      });
      expect(res.status()).toBe(400);
    });

    test('重复用户名返回 409', async ({ request }) => {
      const res = await request.post(`${API_URL}/api/v1/admin/users`, {
        data: {
          username: 'admin',
          password: 'NewUser@123',
          real_name: '重复',
          phone: uniquePhone(),
        },
        headers: authHeaders(token),
      });
      await assertError(res, 10005, 409);
    });
  });

  test.describe('详情', () => {
    test('获取用户详情成功', async ({ request }) => {
      const res = await request.get(`${API_URL}/api/v1/admin/users/1`, {
        headers: getAuthHeaders(token),
      });
      const json = await assertSuccess(res);
      expect(json.data.username).toBe('admin');
      expect(json.data.roles).toContain('系统管理员');
    });

    test('不存在用户返回 404', async ({ request }) => {
      const res = await request.get(`${API_URL}/api/v1/admin/users/99999`, {
        headers: getAuthHeaders(token),
      });
      await assertError(res, 10004, 404);
    });
  });

  test.describe('更新', () => {
    let testUser: { id: number } | null = null;

    test.beforeAll(async ({ request }) => {
      testUser = await createUser(request, {
        username: uniqueName('updatetest'),
        password: 'Update@123',
        real_name: '更新前',
        phone: uniquePhone(),
        role_ids: [4],
      });
    });

    test('更新用户信息成功', async ({ request }) => {
      expect(testUser).not.toBeNull();
      const res = await request.put(`${API_URL}/api/v1/admin/users/${testUser!.id}`, {
        data: { real_name: '更新后', phone: uniquePhone() },
        headers: authHeaders(token),
      });
      await assertSuccess(res);

      const detail = await request.get(`${API_URL}/api/v1/admin/users/${testUser!.id}`, {
        headers: getAuthHeaders(token),
      });
      const json = await assertSuccess(detail);
      expect(json.data.real_name).toBe('更新后');
    });

    test('更新不存在用户返回 404', async ({ request }) => {
      const res = await request.put(`${API_URL}/api/v1/admin/users/99999`, {
        data: { real_name: '不存在', phone: uniquePhone() },
        headers: authHeaders(token),
      });
      await assertError(res, 10004, 404);
    });
  });

  test.describe('冻结/恢复', () => {
    let freezeUser: { id: number; username: string } | null = null;

    test.beforeAll(async ({ request }) => {
      freezeUser = await createUser(request, {
        username: uniqueName('freezetest'),
        password: 'Freeze@123',
        real_name: '冻结测试',
        phone: uniquePhone(),
        role_ids: [4],
      });
    });

    test('冻结用户成功', async ({ request }) => {
      expect(freezeUser).not.toBeNull();
      const res = await request.patch(
        `${API_URL}/api/v1/admin/users/${freezeUser!.id}/freeze`,
        { headers: authHeaders(token) },
      );
      await assertSuccess(res);
    });

    test('不能冻结自己', async ({ request }) => {
      const res = await request.patch(`${API_URL}/api/v1/admin/users/1/freeze`, {
        headers: authHeaders(token),
      });
      await assertError(res, 10003);
    });

    test('重复冻结返回错误', async ({ request }) => {
      expect(freezeUser).not.toBeNull();
      const res = await request.patch(
        `${API_URL}/api/v1/admin/users/${freezeUser!.id}/freeze`,
        { headers: authHeaders(token) },
      );
      await assertError(res, 10006);
    });

    test('恢复用户成功', async ({ request }) => {
      expect(freezeUser).not.toBeNull();
      const res = await request.patch(
        `${API_URL}/api/v1/admin/users/${freezeUser!.id}/unfreeze`,
        { headers: authHeaders(token) },
      );
      await assertSuccess(res);
    });

    test('重复恢复返回错误', async ({ request }) => {
      expect(freezeUser).not.toBeNull();
      const res = await request.patch(
        `${API_URL}/api/v1/admin/users/${freezeUser!.id}/unfreeze`,
        { headers: authHeaders(token) },
      );
      await assertError(res, 10007);
    });
  });
});
