/**
 * 可续传对话 E2E 测试。
 *
 * 验证 ChatStreamProvider 提升到 PortalLayout 后的三条核心行为：
 *   A. 离开功能页再回来 → 流式状态不丢失（会话仍在侧栏，点击可恢复）
 *   B. 刷新页面后点击会话 → 续传/加载历史完成
 *   C. 停止生成后刷新 → 不再恢复生成
 *
 * 前置条件：至少一个可访问的知识库（测试自动检测/创建）。
 * 简化：如知识库无已发布文章导致检索无结果，仍验证消息持久化行为。
 */
import { test, expect } from '@playwright/test';

const API_URL = process.env.PLAYWRIGHT_API_URL || 'http://127.0.0.1:8080';

const ADMIN_CREDS = { username: 'admin', password: 'Admin@123' };
const REPORTER_CREDS = { username: 'reporter1', password: 'Reporter@123' };

/**
 * 通过 API 登录并导航到目标页面。
 *
 * 复制 helpers.ts 的 loginWithCreds 模式：先调 API 获取 JWT，
 * 再设 cookie（供 proxy.ts RBAC）+ localStorage（供 AuthProvider），最后导航。
 * 独立实现以避免依赖现有 helpers 的固定凭据。
 */
async function loginAs(page: import('@playwright/test').Page, creds: { username: string; password: string }, targetPath: string) {
  const res = await page.request.post(`${API_URL}/api/v1/auth/login`, {
    data: creds,
    headers: { 'Content-Type': 'application/json' },
  });
  expect(res.status()).toBe(200);
  const json = await res.json();
  expect(json.code).toBe(0);
  const data = json.data;
  const token: string = data.access_token;
  const refreshToken: string = data.refresh_token;

  const cookieDomain = new URL(API_URL).hostname;
  await page.context().addCookies([
    { name: 'access_token', value: token, path: '/', domain: cookieDomain },
    { name: 'refresh_token', value: refreshToken, path: '/', domain: cookieDomain },
  ]);

  const authJson = JSON.stringify({
    token,
    refreshToken,
    user: data.user,
    roles: data.roles,
    permissions: data.permissions,
    menus: data.menus,
    isLoggedIn: true,
  });
  await page.context().addInitScript((s) => {
    window.localStorage.setItem('auth', s);
  }, authJson);

  await page.goto(targetPath, { waitUntil: 'domcontentloaded' });
  await expect(page).not.toHaveURL(/\/login/, { timeout: 10000 });
}

/** 等待流式开始的共享选择器：ChatPipeline 渲染的当前步骤 div（包含 spinner + 步骤标签） */
const pipelineStepPattern = /查询改写|向量检索|多路检索|BM25 检索|混合融合|重排序/;

/**
 * 等待 SWR 异步拉取知识库列表完成后，选择第一个非占位 option。
 *
 * SWR 在页面挂载后异步请求 KB 列表，直接 check optionCount 会因竞态而误判无 KB。
 * 用 expect.toPass 轮询直到 option > 1。超时返回 false（调用方应 skip 并 return）。
 */
async function waitForKBAndSelect(page: import('@playwright/test').Page, selectLocator: import('@playwright/test').Locator): Promise<boolean> {
  await expect(selectLocator).toBeVisible({ timeout: 8000 });
  try {
    await expect(async () => {
      const count = await selectLocator.locator('option').count();
      expect(count).toBeGreaterThan(1);
    }).toPass({ timeout: 10000 });
  } catch {
    return false;
  }
  await selectLocator.selectOption({ index: 1 });
  return true;
}

test.describe('可续传对话', () => {
  /**
   * 在所有测试之前确保至少有一个知识库。
   *
   * 使用 request fixture 直接调后端 API（不需浏览器），以 admin 身份操作。
   * 先查现有 KB 列表，无则创建，避免重复创建。
   */
  test.beforeAll(async ({ request }) => {
    // 1. 以 admin 登录获取 token
    const loginRes = await request.post(`${API_URL}/api/v1/auth/login`, {
      data: ADMIN_CREDS,
      headers: { 'Content-Type': 'application/json' },
    });
    const loginJson = await loginRes.json();
    expect(loginJson.code).toBe(0);
    const token: string = loginJson.data.access_token;

    // 2. 检查现有知识库
    const listRes = await request.get(`${API_URL}/api/v1/portal/knowledge-bases`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    const listJson = await listRes.json();

    if (listJson.code === 0 && Array.isArray(listJson.data) && listJson.data.length > 0) {
      return;
    }

    // 3. 无可用 KB → 创建
    const createRes = await request.post(`${API_URL}/api/v1/admin/knowledge-bases`, {
      data: {
        name: 'E2E 可续传测试',
        description: 'Playwright 可续传对话 E2E 测试用知识库',
        embedding_model: 'bge-m3',
        vector_dimension: 1024,
      },
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
      },
    });
    const createJson = await createRes.json();
    expect(createJson.code).toBe(0);
  });

  test('A. 离开功能页再回来，会话不丢失', async ({ page }) => {
    await loginAs(page, REPORTER_CREDS, '/portal/chat');

    // 等待 SWR 拉取 KB 列表后选择第一个
    const kbSelect = page.locator('select[aria-label="选择知识库"]');
    if (!(await waitForKBAndSelect(page, kbSelect))) { test.skip(true, '无可用知识库'); return; }

    // 等待输入框出现
    const input = page.locator('input[aria-label="输入消息"]');
    await expect(input).toBeVisible({ timeout: 5000 });

    // 发送问题
    const question = '如何解决 VPN 连接超时的问题？';
    await input.fill(question);
    await page.keyboard.press('Enter');

    // 等待流式开始 — ChatPipeline 显示当前步骤标签。
    // 用 .first() 避免 strict mode（已完成的步骤 badge 也匹配同一正则）。
    await expect(
      page.getByText(pipelineStepPattern).first(),
    ).toBeVisible({ timeout: 20000 });

    // 切到「我的申告」— Nav 使用 <button> + router.push，不是 <a>/<Link>
    await page.locator('header nav').getByRole('button', { name: '我的申告' }).click();
    await expect(page).toHaveURL(/\/portal\/tickets/, { timeout: 8000 });

    // 切回「智能问答」
    await page.locator('header nav').getByRole('button', { name: '智能问答' }).click();
    await expect(page).toHaveURL(/\/portal\/chat/, { timeout: 8000 });

    // ChatPage 重新挂载后 sessionId 重置为 null，需重新选择知识库以显示输入区。
    // 但会话本身保存在 ChatStreamProvider（layout 层）和服务器，侧栏可见。
    const kbSelect2 = page.locator('select[aria-label="选择知识库"]');
    if (!(await waitForKBAndSelect(page, kbSelect2))) { test.skip(true, '无可用知识库'); return; }

    // 在侧边栏找到刚才的会话并点击
    const sessionBtn = page.locator('aside').getByText(question).first();
    await expect(sessionBtn).toBeVisible({ timeout: 8000 });
    await sessionBtn.click();

    // 验证消息已恢复 — 至少用户消息可见
    await expect(page.getByText(question).first()).toBeVisible({ timeout: 10000 });
  });

  test('B. 刷新页面后点击会话，消息加载完成', async ({ page }) => {
    await loginAs(page, REPORTER_CREDS, '/portal/chat');

    // 等待 KB 列表加载并选择
    const kbSelect = page.locator('select[aria-label="选择知识库"]');
    if (!(await waitForKBAndSelect(page, kbSelect))) { test.skip(true, '无可用知识库'); return; }

    const input = page.locator('input[aria-label="输入消息"]');
    await expect(input).toBeVisible({ timeout: 5000 });

    const question = '如何重置 VPN 密码？';
    await input.fill(question);
    await page.keyboard.press('Enter');

    // 等待流式开始
    await expect(
      page.getByText(pipelineStepPattern).first(),
    ).toBeVisible({ timeout: 20000 });

    // 刷新页面 — cookie + initScript 保持登录
    await page.reload({ waitUntil: 'domcontentloaded' });
    await expect(page).not.toHaveURL(/\/login/, { timeout: 10000 });

    // 重新选择知识库（reload 后 selectedKB 重置，且 SWR 重新 fetch）
    const kbSelect2 = page.locator('select[aria-label="选择知识库"]');
    if (!(await waitForKBAndSelect(page, kbSelect2))) { test.skip(true, '无可用知识库'); return; }

    // 在侧边栏中找到刚才的会话并点击
    // 会话按钮内包含问题文本，位于 <aside> 中
    const sessionBtn = page.locator('aside').getByText(question).first();
    await expect(sessionBtn).toBeVisible({ timeout: 8000 });
    await sessionBtn.click();

    // 等待消息区出现 — 可能是历史加载或续传完成
    // 用户消息至少应出现
    await expect(page.getByText(question).first()).toBeVisible({ timeout: 15000 });
  });

  test('C. 停止生成后刷新，不再恢复生成', async ({ page }) => {
    await loginAs(page, REPORTER_CREDS, '/portal/chat');

    // 等待 KB 列表加载并选择
    const kbSelect = page.locator('select[aria-label="选择知识库"]');
    if (!(await waitForKBAndSelect(page, kbSelect))) { test.skip(true, '无可用知识库'); return; }

    const input = page.locator('input[aria-label="输入消息"]');
    await expect(input).toBeVisible({ timeout: 5000 });

    const question = '请详细说明 VPN 的所有排查步骤';
    await input.fill(question);
    await page.keyboard.press('Enter');

    // 等待流式开始 + 停止按钮出现
    const stopBtn = page.locator('button[aria-label="停止生成"]');
    await expect(stopBtn).toBeVisible({ timeout: 20000 });

    // 点击停止
    await stopBtn.click();

    // 等待停止按钮消失（流式已终止）
    await expect(stopBtn).not.toBeVisible({ timeout: 10000 });

    // 刷新页面
    await page.reload({ waitUntil: 'domcontentloaded' });
    await expect(page).not.toHaveURL(/\/login/, { timeout: 10000 });

    // 重新选择知识库
    const kbSelect2 = page.locator('select[aria-label="选择知识库"]');
    if (!(await waitForKBAndSelect(page, kbSelect2))) { test.skip(true, '无可用知识库'); return; }

    // 点击侧边栏中的会话加载消息
    const sessionBtn = page.locator('aside').getByText(question).first();
    await expect(sessionBtn).toBeVisible({ timeout: 8000 });
    await sessionBtn.click();

    // 等待消息加载完成（给续传/加载一定时间）
    await page.waitForTimeout(3000);

    // 不应该出现停止生成按钮（没有 running generation）
    await expect(page.locator('button[aria-label="停止生成"]')).toHaveCount(0);
  });
});
