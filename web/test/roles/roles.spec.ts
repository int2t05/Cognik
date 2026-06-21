/**
 * 角色管理 E2E 测试。
 */
import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('角色管理', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page, '/admin/roles');
  });

  test('显示标题', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '角色管理' })).toBeVisible({ timeout: 5000 });
  });

  test('表格渲染', async ({ page }) => {
    await expect(page.locator('table')).toBeVisible({ timeout: 5000 });
  });
});
