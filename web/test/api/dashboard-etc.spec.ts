/**
 * 数据看板 & 审计日志 & LLM 配置 API 集成测试。
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
} from './helpers';

test.describe('数据看板 API', () => {
  let token = '';

  test.beforeAll(async ({ request }) => {
    const auth = await loginAsAdmin(request);
    token = auth.accessToken;
  });

  test('获取统计数据', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/v1/admin/dashboard/stats`, {
      headers: getAuthHeaders(token),
    });
    const json = await assertSuccess(res);
    expect(json.data).toHaveProperty('today_tickets');
    expect(json.data).toHaveProperty('knowledge_count');
  });

  test('获取趋势数据', async ({ request }) => {
    const today = new Date().toISOString().slice(0, 10);
    const startDate = new Date(Date.now() - 30 * 86400000).toISOString().slice(0, 10);
    const res = await request.get(
      `${API_URL}/api/v1/admin/dashboard/trends?start_date=${startDate}&end_date=${today}`,
      { headers: getAuthHeaders(token) },
    );
    const json = await assertSuccess(res);
    expect(json.data).toHaveProperty('data_points');
    expect(Array.isArray(json.data.data_points)).toBe(true);
  });

  test('无 token 返回 401', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/v1/admin/dashboard/stats`);
    await assertError(res, 10001, 401);
  });
});

test.describe('审计日志 API', () => {
  let token = '';

  test.beforeAll(async ({ request }) => {
    const auth = await loginAsAdmin(request);
    token = auth.accessToken;
  });

  test('获取审计日志列表', async ({ request }) => {
    const res = await request.get(
      `${API_URL}/api/v1/admin/audit-logs?page=1&page_size=10`,
      { headers: getAuthHeaders(token) },
    );
    const json = await assertSuccess(res);
    expect(Array.isArray(json.data)).toBe(true);
    expect(json).toHaveProperty('total');
  });

  test('审计日志分页', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/v1/admin/audit-logs?page=1&page_size=5`, {
      headers: getAuthHeaders(token),
    });
    const body = await assertSuccess(res);
    expect(body.data.total).toBeGreaterThanOrEqual(0);
    expect(body.data.page).toBe(1);
    expect(body.data.page_size).toBe(5);
  });

  test('按日期筛选', async ({ request }) => {
    const today = new Date().toISOString().slice(0, 10);
    const res = await request.get(
      `${API_URL}/api/v1/admin/audit-logs?page=1&page_size=10&date_from=${today}&date_to=${today}`,
      { headers: getAuthHeaders(token) },
    );
    await assertSuccess(res);
  });

  test('无 token 返回 401', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/v1/admin/audit-logs?page=1`);
    await assertError(res, 10001, 401);
  });
});

test.describe('LLM 配置 API', () => {
  let token = '';
  let configId = 0;

  test.beforeAll(async ({ request }) => {
    const auth = await loginAsAdmin(request);
    token = auth.accessToken;
  });

  test('获取 LLM 配置列表', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/v1/admin/llm-configs`, {
      headers: getAuthHeaders(token),
    });
    const json = await assertSuccess(res);
    expect(Array.isArray(json.data)).toBe(true);
  });

  test('创建 LLM 配置', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/v1/admin/llm-configs`, {
      data: {
        name: uniqueName('llm-test'),
        provider_type: 1, // OpenAI-compatible
        base_url: 'https://api.openai.com/v1',
        model: 'gpt-4o-mini',
        llm_model: 'gpt-4o-mini',
        embedding_model: 'text-embedding-3-small',
        api_key: 'sk-test-key-123',
        is_default: false,
      },
      headers: authHeaders(token),
    });
    const json = await assertSuccess(res);
    configId = json.data?.id;
  });

  test('LLM 配置详情', async ({ request }) => {
    if (!configId) test.skip();
    const res = await request.get(`${API_URL}/api/v1/admin/llm-configs/${configId}`, {
      headers: getAuthHeaders(token),
    });
    await assertSuccess(res);
  });

  test('更新 LLM 配置', async ({ request }) => {
    if (!configId) test.skip();
    const res = await request.put(`${API_URL}/api/v1/admin/llm-configs/${configId}`, {
      data: {
        name: uniqueName('llm-updated'),
        provider_type: 1,
        base_url: 'https://api.openai.com/v1',
        model: 'gpt-4o',
        llm_model: 'gpt-4o',
        embedding_model: 'text-embedding-3-small',
      },
      headers: authHeaders(token),
    });
    await assertSuccess(res);
  });

  test('删除 LLM 配置', async ({ request }) => {
    if (!configId) test.skip();
    const res = await request.delete(`${API_URL}/api/v1/admin/llm-configs/${configId}`, {
      headers: getAuthHeaders(token),
    });
    await assertSuccess(res);

    const detail = await request.get(`${API_URL}/api/v1/admin/llm-configs/${configId}`, {
      headers: getAuthHeaders(token),
    });
    await assertError(detail, 10004, 404);
  });

  // 清理测试创建的 LLM 配置
  test.afterAll(async ({ request }) => {
    const auth = await loginAsAdmin(request);
    const t = auth.accessToken;
    const res = await request.get(`${API_URL}/api/v1/admin/llm-configs`, {
      headers: getAuthHeaders(t),
    });
    const body = await res.json();
    if (Array.isArray(body.data)) {
      for (const cfg of body.data) {
        if (cfg.name && (cfg.name.startsWith('llm-test') || cfg.name.startsWith('llm-updated'))) {
          await request.delete(`${API_URL}/api/v1/admin/llm-configs/${cfg.id}`, {
            headers: getAuthHeaders(t),
          });
        }
      }
    }
  });
});
