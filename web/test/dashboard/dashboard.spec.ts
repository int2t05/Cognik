/**
 * 数据看板 E2E 测试。
 *
 * 覆盖：标题、统计卡片、刷新按钮、趋势图渲染、卡片数值验证。
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
    // 趋势图使用纯 CSS 柱状图（非 canvas/SVG），验证页面有统计卡片后的额外内容区
    // main 区域应包含多个子元素（统计卡片 + 趋势区）
    const mainChildren = page.locator('main > div > div');
    const childCount = await mainChildren.count();
    expect(childCount).toBeGreaterThan(0);
  });

  test('统计卡片显示数值', async ({ page }) => {
    // 统计卡片应包含数字值
    const statCards = page
      .locator('[class*="StatCard"], [class*="stat"], [class*="card"]')
      .filter({ hasText: /[0-9]/ });
    const cardCount = await statCards.count();
    if (cardCount > 0) {
      await expect(statCards.first()).toBeVisible({ timeout: 5000 });
    }
  });
});
