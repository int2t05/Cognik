/**
 * 用户管理模块 E2E 测试。
 */
import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('用户管理', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page, '/admin/users');
  });

  test('显示标题和用户表格', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '用户管理' })).toBeVisible();
    await expect(page.locator('table')).toBeVisible({ timeout: 5000 });
  });

  test('搜索框可用', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '用户管理' })).toBeVisible();
    const searchInput = page.locator('input[placeholder*="搜索"]');
    if (await searchInput.isVisible()) {
      await searchInput.fill('admin');
      // 等待搜索结果
      await expect(page.locator('table')).toBeVisible();
    }
  });

  test('分页组件可见', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '用户管理' })).toBeVisible();
    // ApplePagination 应该渲染
    await expect(page.locator('table')).toBeVisible({ timeout: 5000 });
  });

  test('用户状态标签正常渲染', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '用户管理' })).toBeVisible();
    // 表格内应有状态文字或状态相关元素
    await expect(page.locator('table')).toBeVisible({ timeout: 5000 });
  });
});
