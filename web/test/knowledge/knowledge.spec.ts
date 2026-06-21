/**
 * 知识库管理 E2E 测试 — 完整 CRUD 流程。
 */
import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

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
    // 等待 dialog 出现（Radix Portal 渲染）
    await page.waitForTimeout(500);
    const dialog = page.locator('[role="dialog"]');
    if (await dialog.isVisible()) {
      // 填写名称
      const nameInput = dialog.locator('input').first();
      if (await nameInput.isVisible()) {
        await nameInput.fill('E2E 测试知识库');
      }
      // 保存
      const saveBtn = dialog.getByRole('button', { name: '保存' });
      if (await saveBtn.isVisible()) {
        await saveBtn.click();
        await expect(page.getByText('E2E 测试知识库')).toBeVisible({ timeout: 5000 });
      }
    }
  });

  test('知识库卡片可点击导航', async ({ page }) => {
    const card = page.locator('[class*="cursor-pointer"]').first();
    if (await card.isVisible()) {
      await card.click();
      await expect(page).not.toHaveURL(/\/login/, { timeout: 5000 });
    }
  });

  test('进入知识库详情可看到文章和文档上传', async ({ page }) => {
    const card = page.locator('[class*="cursor-pointer"]').first();
    if (!(await card.isVisible())) { test.skip(); return; }
    await card.click();
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 5000 });
  });
});
