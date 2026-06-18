import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('门户端申告', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page, '/portal/tickets');
  });

  test('列表显示标题', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '我的申告' })).toBeVisible();
  });

  test('新建页面可访问', async ({ page }) => {
    await page.goto('/portal/tickets/new');
    await expect(page.getByRole('main').getByRole('button', { name: /提交/ })).toBeVisible({ timeout: 5000 });
  });
});

test.describe('后台申告管理', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page, '/admin/tickets');
  });

  test('列表显示筛选', async ({ page }) => {
    await expect(page.getByText('申告管理')).toBeVisible();
    await expect(page.getByText('全部')).toBeVisible();
  });
});
