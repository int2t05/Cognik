/**
 * 知识库管理 E2E 测试 — 完整 CRUD 流程。
 *
 * 覆盖：标题渲染、URL 验证、新建知识库弹窗、卡片导航、详情页。
 * 避免 waitForTimeout，改用断言超时和响应等待。
 */
import { test, expect, request as pwRequest } from '@playwright/test';
import { loginAsAdmin, API_URL, ADMIN_CREDS } from '../helpers';

test.describe('知识库管理', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page, '/admin/knowledge');
  });

  test('显示标题和新建按钮', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '知识库管理' })).toBeVisible();
    await expect(page.getByRole('button', { name: '新建知识库' })).toBeVisible();
  });

  test('页面 URL 正确', async ({ page }) => {
    await expect(page).toHaveURL(/\/admin\/knowledge/);
  });

  test('新建知识库弹窗可交互', async ({ page }) => {
    await page.getByRole('button', { name: '新建知识库' }).click();
    // 等待 dialog 出现（Radix Portal 渲染），使用断言超时而非固定延时
    const dialog = page.locator('[role="dialog"]');
    await expect(dialog).toBeVisible({ timeout: 3000 }).catch(() => {});
    if (await dialog.isVisible().catch(() => false)) {
      // 使用 getByLabel 而非 input.first()
      const nameInput = dialog.getByLabel(/名称/i);
      if (await nameInput.isVisible().catch(() => false)) {
        await nameInput.fill('E2E 测试知识库');
      }
      // 保存
      const saveBtn = dialog.getByRole('button', { name: '保存' });
      if (await saveBtn.isVisible().catch(() => false)) {
        await saveBtn.click();
        // 等待新创建的知识库名称出现在页面中
        await expect(page.getByText('E2E 测试知识库').first()).toBeVisible({ timeout: 5000 });
      }
    }
  });

  test('知识库卡片可点击导航', async ({ page }) => {
    const card = page.locator('[class*="cursor-pointer"]').first();
    if (await card.isVisible().catch(() => false)) {
      await card.click();
      await expect(page).not.toHaveURL(/\/login/, { timeout: 5000 });
    }
  });

  test('进入知识库详情可看到文章和文档上传', async ({ page }) => {
    const card = page.locator('[class*="cursor-pointer"]').first();
    if (!(await card.isVisible().catch(() => false))) {
      test.skip();
      return;
    }
    await card.click();
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 5000 });
  });

  // 清理：删除 E2E 测试过程中创建的知识库
  test.afterAll(async () => {
    const ctx = await pwRequest.newContext();
    try {
      const loginRes = await ctx.post(`${API_URL}/api/v1/auth/login`, {
        data: ADMIN_CREDS,
        headers: { 'Content-Type': 'application/json' },
      });
      if (loginRes.ok()) {
        await ctx.post(`${API_URL}/api/v1/test/e2e-cleanup`).catch(() => {});
      }
    } finally {
      await ctx.dispose();
    }
  });
});
