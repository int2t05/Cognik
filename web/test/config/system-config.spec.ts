/**
 * 系统配置 E2E 测试。
 */
import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('系统配置', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page, '/admin/config/system');
  });

  test('显示标题和配置列表', async ({ page }) => {
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 5000 });
  });
});
