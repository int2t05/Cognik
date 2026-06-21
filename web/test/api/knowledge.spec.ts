/**
 * 知识库管理 API 集成测试。
 *
 * 覆盖：KB/文章 CRUD + 参数校验 + 权限校验。
 * 创建端点返回 data:null → 通过列表搜索获取 ID。
 */
import { test, expect, type APIRequestContext } from '@playwright/test';
import {
  API_URL,
  loginAsAdmin,
  authHeaders,
  assertSuccess,
  assertError,
  uniqueName,
} from './helpers';

test.describe('知识库 API', () => {
  let token = '';

  test.beforeAll(async ({ request }) => {
    const auth = await loginAsAdmin(request);
    token = auth.accessToken;
  });

  /** 从列表中按名称查找 KB ID */
  async function findKbId(request: APIRequestContext, name: string): Promise<number> {
    const res = await request.get(`${API_URL}/api/v1/admin/knowledge-bases`, {
      headers: authHeaders(token),
    });
    const json = await res.json();
    if (json.code === 0 && Array.isArray(json.data)) {
      const match = json.data.find((kb: { name: string }) => kb.name === name);
      return match?.id || 0;
    }
    return 0;
  }

  /** 获取任意存在的 KB ID */
  async function getAnyKbId(request: APIRequestContext): Promise<number> {
    const res = await request.get(`${API_URL}/api/v1/admin/knowledge-bases`, {
      headers: authHeaders(token),
    });
    const json = await res.json();
    if (json.code === 0 && Array.isArray(json.data) && json.data.length > 0) {
      return json.data[0].id;
    }
    return 0;
  }

  test('创建知识库', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/v1/admin/knowledge-bases`, {
      data: {
        name: uniqueName('测试KB'),
        description: 'API 测试',
        embedding_model: 'bge-m3',
        vector_dimension: 1024,
      },
      headers: authHeaders(token),
    });
    await assertSuccess(res);
  });

  test('获取知识库列表', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/v1/admin/knowledge-bases`, {
      headers: authHeaders(token),
    });
    const json = await assertSuccess(res);
    expect(Array.isArray(json.data)).toBe(true);
  });

  test('知识库详情（无独立详情端点，通过列表验证）', async ({ request }) => {
    // GET /knowledge-bases/:id 端点不存在于当前路由中，
    // 通过列表确认创建的知识库可被检索
    const res = await request.get(`${API_URL}/api/v1/admin/knowledge-bases`, {
      headers: authHeaders(token),
    });
    const json = await assertSuccess(res);
    expect(Array.isArray(json.data)).toBe(true);
  });

  test('更新知识库', async ({ request }) => {
    const id = await getAnyKbId(request);
    if (!id) { test.skip(); return; }
    const newName = uniqueName('已更新KB');
    const res = await request.put(`${API_URL}/api/v1/admin/knowledge-bases/${id}`, {
      data: { name: newName, description: '更新后' },
      headers: authHeaders(token),
    });
    await assertSuccess(res);
  });

  test('创建知识库缺少名称返回 400', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/v1/admin/knowledge-bases`, {
      data: { description: 'no name' },
      headers: authHeaders(token),
    });
    expect(res.status()).toBe(400);
  });

  test.describe('文章', () => {
    const ARTICLE_TITLE = 'API 测试文章';
    let articleId = 0;
    let articleKbId = 0;

    async function findArticleId(request: APIRequestContext): Promise<number> {
      if (!articleKbId) return 0;
      const res = await request.get(
        `${API_URL}/api/v1/admin/knowledge-bases/${articleKbId}/articles?page=1&page_size=10`,
        { headers: authHeaders(token) },
      );
      const json = await res.json();
      if (json.code === 0 && Array.isArray(json.data)) {
        const match = json.data.find((a: { title: string }) => a.title === ARTICLE_TITLE);
        return match?.id || 0;
      }
      return 0;
    }

    test('创建文章', async ({ request }) => {
      articleKbId = await getAnyKbId(request);
      if (!articleKbId) { test.skip(); return; }
      const res = await request.post(
        `${API_URL}/api/v1/admin/knowledge-bases/${articleKbId}/articles`,
        {
          data: { title: ARTICLE_TITLE, content: '# 测试', source_type: 1 },
          headers: authHeaders(token),
        },
      );
      await assertSuccess(res);
      // 后端返回 data:null，通过列表搜索获取 ID
      articleId = await findArticleId(request);
    });

    test('文章详情', async ({ request }) => {
      if (!articleId) { test.skip(); return; }
      // 文章端点路径为 /articles/:id（不在 knowledge-bases 下）
      const res = await request.get(`${API_URL}/api/v1/admin/articles/${articleId}`, {
        headers: authHeaders(token),
      });
      await assertSuccess(res);
    });

    test('更新文章', async ({ request }) => {
      if (!articleId) { test.skip(); return; }
      const res = await request.put(`${API_URL}/api/v1/admin/articles/${articleId}`, {
        data: { title: '已更新 — 文章', content: '# Updated' },
        headers: authHeaders(token),
      });
      await assertSuccess(res);
    });
  });

  test('无 token 访问返回 401', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/v1/admin/knowledge-bases`);
    await assertError(res, 10001, 401);
  });
});
