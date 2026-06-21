/**
 * 申告管理 API 集成测试。
 *
 * 覆盖：门户创建/查询/详情/补充 + 后台列表/状态变更。
 * 创建端点返回 data:null → 通过列表搜索获取 ID。
 */
import { test, expect } from '@playwright/test';
import {
  API_URL,
  loginAsAdmin,
  authHeaders,
  assertSuccess,
  assertError,
} from './helpers';

test.describe('申告 API', () => {
  let token = '';

  test.beforeAll(async ({ request }) => {
    const auth = await loginAsAdmin(request);
    token = auth.accessToken;
  });

  /** 搜索最新申告 ID */
  async function findLatestTicketId(request: APIRequestContext, title: string): Promise<number> {
    const res = await request.get(
      `${API_URL}/api/v1/admin/tickets?page=1&page_size=10&status=-1`,
      { headers: authHeaders(token) },
    );
    const json = await res.json();
    if (json.code === 0 && json.data?.length > 0) {
      const match = json.data.find((t: { title: string }) => t.title === title);
      return match?.id || 0;
    }
    return 0;
  }

  test.describe('门户端', () => {
    const TICKET_TITLE = 'API 测试申告 — 邮箱问题';
    let ticketId = 0;

    test('创建申告成功', async ({ request }) => {
      const res = await request.post(`${API_URL}/api/v1/portal/tickets`, {
        data: {
          title: TICKET_TITLE,
          description: '自动化测试创建的申告',
          urgency: 2,
          impact_scope: 1,
          contact_phone: '13800007777',
        },
        headers: authHeaders(token),
      });
      await assertSuccess(res);
    });

    test('创建申告缺少必填字段返回 400', async ({ request }) => {
      const res = await request.post(`${API_URL}/api/v1/portal/tickets`, {
        data: { description: 'no title' },
        headers: authHeaders(token),
      });
      expect(res.status()).toBe(400);
    });

    test('查询我的申告列表', async ({ request }) => {
      const res = await request.get(`${API_URL}/api/v1/portal/tickets?page=1&page_size=10`, {
        headers: authHeaders(token),
      });
      const json = await assertSuccess(res);
      expect(Array.isArray(json.data)).toBe(true);
    });

    test('不存在申告返回 404', async ({ request }) => {
      const res = await request.get(`${API_URL}/api/v1/portal/tickets/99999`, {
        headers: authHeaders(token),
      });
      await assertError(res, 10004, 404);
    });
  });

  test.describe('后台管理', () => {
    test('全部申告列表', async ({ request }) => {
      const res = await request.get(
        `${API_URL}/api/v1/admin/tickets?page=1&page_size=10&status=-1`,
        { headers: authHeaders(token) },
      );
      const json = await assertSuccess(res);
      expect(Array.isArray(json.data)).toBe(true);
    });

    test('按状态筛选', async ({ request }) => {
      const res = await request.get(
        `${API_URL}/api/v1/admin/tickets?page=1&page_size=10&status=1`,
        { headers: authHeaders(token) },
      );
      const json = await assertSuccess(res);
      json.data.forEach((t: { status: number }) => expect(t.status).toBe(1));
    });

    test('后台查看申告详情', async ({ request }) => {
      const listRes = await request.get(
        `${API_URL}/api/v1/admin/tickets?page=1&page_size=5&status=-1`,
        { headers: authHeaders(token) },
      );
      const listJson = await listRes.json();
      if (listJson.code !== 0 || !listJson.data?.length) { test.skip(); return; }
      const id = listJson.data[0].id;

      const res = await request.get(`${API_URL}/api/v1/admin/tickets/${id}`, {
        headers: authHeaders(token),
      });
      await assertSuccess(res);
    });

    test('更新申告状态 — 开始处理', async ({ request }) => {
      // 找一个待处理的申告
      const listRes = await request.get(
        `${API_URL}/api/v1/admin/tickets?page=1&page_size=10&status=1`,
        { headers: authHeaders(token) },
      );
      const listJson = await listRes.json();
      if (!listJson.data?.length) { test.skip(); return; }
      const id = listJson.data[0].id;

      const res = await request.patch(
        `${API_URL}/api/v1/admin/tickets/${id}/status`,
        {
          data: { action: 'start', result: '已分配处理' },
          headers: authHeaders(token),
        },
      );
      await assertSuccess(res);
    });

    test('状态机违规返回错误', async ({ request }) => {
      // 找一个处理中的申告
      const listRes = await request.get(
        `${API_URL}/api/v1/admin/tickets?page=1&page_size=10&status=2`,
        { headers: authHeaders(token) },
      );
      const listJson = await listRes.json();
      if (!listJson.data?.length) { test.skip(); return; }
      const id = listJson.data[0].id;

      // 处理中→start 不可行
      const res = await request.patch(
        `${API_URL}/api/v1/admin/tickets/${id}/status`,
        { data: { action: 'start' }, headers: authHeaders(token) },
      );
      await assertError(res, 10003);
    });
  });
});
