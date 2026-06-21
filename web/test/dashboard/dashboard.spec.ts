/**
 * 数据看板 E2E 测试。
 */
import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('数据看板', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page, '/admin/dashboard');
  });

  test('显示标题和统计卡片', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '数据看板' })).toBeVisible();
    await expect(page.getByText(/今日申告|今日问答|知识条目/).first()).toBeVisible({ timeout: 5000 });
  });

  test('刷新按钮可用', async ({ page }) => {
    await expect(page.getByRole('button', { name: '刷新' })).toBeVisible();
  });

  test('30 日趋势区域可渲染', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '30 日趋势' })).toBeVisible();
    // 趋势图容器存在（可能是图表或空状态）
    await expect(page.locator('[role="img"], .bg-\\[var\\(--color-canvas\\)\\]').first()).toBeVisible({ timeout: 5000 });
  });
});
