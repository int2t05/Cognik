/**
 * 知识库管理模块 E2E 测试。
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

  test('新建知识库弹窗可打开', async ({ page }) => {
    await page.getByRole('button', { name: '新建知识库' }).click();
    // Dialog 应出现，包含保存按钮
    await expect(page.getByRole('button', { name: '保存' })).toBeVisible({ timeout: 3000 });
    // 关闭
    await page.getByRole('button', { name: '取消' }).click();
  });

  test('知识库卡片可点击导航', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '知识库管理' })).toBeVisible();
    // 如果有知识库卡片，点击第一个
    const cards = page.locator('[class*="cursor-pointer"]');
    const count = await cards.count();
    if (count > 0) {
      await cards.first().click();
      // 应导航到详情页
      await expect(page).toHaveURL(/\/admin\/knowledge\/\d+/, { timeout: 5000 });
    }
  });
});
