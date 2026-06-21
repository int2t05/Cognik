/**
 * 门户消息 E2E 测试。
 */
import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('门户消息', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page, '/portal/messages');
  });

  test('显示标题', async ({ page }) => {
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 5000 });
  });

  test('页面内容渲染', async ({ page }) => {
    // 消息列表或空状态提示
    await expect(page.locator('main').first()).toBeVisible({ timeout: 5000 });
  });
});
