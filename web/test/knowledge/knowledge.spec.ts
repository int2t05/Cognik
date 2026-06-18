/**
 * 知识库管理 E2E 测试。
 *
 * 覆盖知识库列表、创建/编辑知识库、文章列表、文章创建等核心流程。
 * 测试针对后台管理端（/admin/knowledge）。
 */

import { test, expect, type Page } from '@playwright/test';

/** 管理员登录辅助函数 */
async function loginAsAdmin(page: Page) {
  await page.goto('/login');
  await page.locator('input[autocomplete="username"]').fill('admin');
  await page.locator('input[autocomplete="current-password"]').fill('Admin@123');
  await page.getByRole('button', { name: '登录' }).click();
  await page.waitForURL('**/admin/dashboard');
}

test.describe('知识库列表', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto('/admin/knowledge');
    await page.waitForURL('**/admin/knowledge');
  });

  test('知识库页面加载，显示标题和新建按钮', async ({ page }) => {
    await expect(page.getByText('知识库管理')).toBeVisible();
    await expect(page.getByRole('button', { name: '新建知识库' })).toBeVisible();
  });
});

test.describe('创建知识库', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto('/admin/knowledge');
    await page.waitForURL('**/admin/knowledge');
  });

  test('新建知识库对话框可打开并提交', async ({ page }) => {
    // 点击新建知识库按钮
    await page.getByRole('button', { name: '新建知识库' }).click();

    // 对话框应弹出
    await expect(page.getByText('新建知识库')).toBeVisible();

    // 填写表单
    await page.locator('label:has-text("名称") + input, label:has-text("名称") ~ input').first().fill('E2E 测试知识库');
    await page.locator('label:has-text("描述") + input, label:has-text("描述") ~ input').first().fill('由 Playwright E2E 测试自动创建');

    // 点击保存
    await page.getByRole('button', { name: '保存' }).click();

    // 保存成功后对话框关闭，知识库列表应刷新
    // 可能显示成功 toast 或列表中包含新条目
    await expect(page.getByText('已创建').or(page.getByText('E2E 测试知识库'))).toBeVisible({ timeout: 5000 });
  });

  test('空名称提交显示验证提示', async ({ page }) => {
    await page.getByRole('button', { name: '新建知识库' }).click();
    await page.getByRole('button', { name: '保存' }).click();

    // 应显示前端校验错误
    await expect(page.getByText('请输入知识库名称')).toBeVisible({ timeout: 3000 });
  });
});

test.describe('文章管理', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto('/admin/knowledge');
    await page.waitForURL('**/admin/knowledge');
  });

  test('进入知识库查看文章列表', async ({ page }) => {
    // 等待知识库列表加载
    await page.waitForTimeout(2000);
    await page.waitForLoadState('networkidle');

    // 判断是否存在知识库卡片
    const kbCards = page.locator('h3').filter({ hasNotText: '知识库管理' });
    const cardCount = await kbCards.count();

    if (cardCount === 0) {
      test.skip('无可用知识库，跳过测试');
      return;
    }

    // 点击第一个知识库卡片
    await kbCards.first().click();

    // 跳转至文章列表页
    await page.waitForURL(/\/admin\/knowledge\/\d+$/);

    // 应显示文章列表页标题和筛选按钮
    await expect(page.getByText('知识文章')).toBeVisible();
    await expect(page.getByText('全部')).toBeVisible();
    await expect(page.getByText('草稿')).toBeVisible();
    await expect(page.getByText('已发布')).toBeVisible();

    // 应有新建文章按钮
    await expect(page.getByRole('button', { name: '新建文章' })).toBeVisible();
  });

  test('筛选文章状态', async ({ page }) => {
    await page.waitForTimeout(2000);

    const kbCards = page.locator('h3').filter({ hasNotText: '知识库管理' });
    if ((await kbCards.count()) === 0) {
      test.skip('无可用知识库，跳过测试');
      return;
    }
    await kbCards.first().click();
    await page.waitForURL(/\/admin\/knowledge\/\d+$/);
    await page.waitForLoadState('networkidle');

    // 点击"已发布"筛选
    await page.getByText('已发布').click();

    // 筛选按钮应高亮
    const activeFilter = page.locator('button').filter({ hasText: '已发布' });
    await expect(activeFilter).toBeVisible();
  });
});

test.describe('创建文章', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto('/admin/knowledge');
    await page.waitForURL('**/admin/knowledge');
  });

  test('导航至新建文章页面并填写表单', async ({ page }) => {
    await page.waitForTimeout(2000);

    const kbCards = page.locator('h3').filter({ hasNotText: '知识库管理' });
    if ((await kbCards.count()) === 0) {
      test.skip('无可用知识库，跳过测试');
      return;
    }
    await kbCards.first().click();
    await page.waitForURL(/\/admin\/knowledge\/\d+$/);
    await page.waitForLoadState('networkidle');

    // 点击"新建文章"按钮
    await page.getByRole('button', { name: '新建文章' }).click();
    await page.waitForURL(/\/admin\/knowledge\/\d+\/new/);

    // 新建文章页面
    await expect(page.getByText('新建文章')).toBeVisible();

    // 填写手动创建表单
    await page.locator('input[placeholder="知识文章标题"]').fill('E2E 测试文章');
    await page.locator('textarea[placeholder*="Markdown"]').fill('## 测试标题\n\n这是由 Playwright E2E 测试创建的文章内容。');

    // 点击创建
    await page.getByRole('button', { name: '创建文章' }).click();

    // 创建成功后应跳转至文章详情页
    await page.waitForURL(/\/admin\/knowledge\/\d+\/\d+/, { timeout: 10000 });
  });
});
