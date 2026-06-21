/**
 * 数据看板 & 审计日志 & LLM 配置 API 集成测试。
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

test.describe('数据看板 API', () => {
  let token = '';

  test.beforeAll(async ({ request }) => {
    const auth = await loginAsAdmin(request);
    token = auth.accessToken;
  });

  test('获取统计数据', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/v1/admin/dashboard/stats`, {
      headers: authHeaders(token),
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
      { headers: authHeaders(token) },
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
      { headers: authHeaders(token) },
    );
    const json = await assertSuccess(res);
    expect(Array.isArray(json.data)).toBe(true);
    expect(json).toHaveProperty('total');
  });

  test('按日期筛选', async ({ request }) => {
    const today = new Date().toISOString().slice(0, 10);
    const res = await request.get(
      `${API_URL}/api/v1/admin/audit-logs?page=1&page_size=10&date_from=${today}&date_to=${today}`,
      { headers: authHeaders(token) },
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
      headers: authHeaders(token),
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
      headers: authHeaders(token),
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
      headers: authHeaders(token),
    });
    await assertSuccess(res);

    const detail = await request.get(`${API_URL}/api/v1/admin/llm-configs/${configId}`, {
      headers: authHeaders(token),
    });
    await assertError(detail, 10004, 404);
  });
});
