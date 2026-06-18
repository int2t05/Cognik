/**
 * 认证模块 E2E 测试。
 */
import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('认证', () => {
  test('有效凭证登录后可访问受保护页面', async ({ page }) => {
    await loginAsAdmin(page);
    await expect(page).not.toHaveURL(/\/login/);
  });

  test('登录页表单可交互', async ({ page }) => {
    await page.goto('/login');
    // 验证登录表单元素可见
    await expect(page.locator('input[autocomplete="username"]')).toBeVisible();
    await expect(page.locator('input[autocomplete="current-password"]')).toBeVisible();
    await expect(page.getByRole('button', { name: '登录' })).toBeVisible();
    // 空表单点击 — toast 应显示错误
    await page.getByRole('button', { name: '登录' }).click();
    // 验证页面未跳转（空表单不应触发登录）
    await expect(page).toHaveURL(/\/login/);
    await expect(page.getByRole('button', { name: '登录' })).toBeVisible();
  });

  test('未登录访问受保护页面重定向到登录', async ({ page }) => {
    await page.goto('/portal/chat');
    await expect(page).toHaveURL(/\/login/, { timeout: 10000 });
  });
});
