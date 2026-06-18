import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('用户列表', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page, '/admin/users');
  });

  test('显示标题和表格', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '用户管理' })).toBeVisible();
    await expect(page.locator('table')).toBeVisible({ timeout: 5000 });
  });

  test('搜索框可用', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '用户管理' })).toBeVisible();
    await expect(page.locator('input[placeholder*="搜索"]')).toBeVisible();
    await expect(page).toHaveURL(/\/admin\/users/);
  });
});
