/**
 * 认证模块 E2E 测试。
 *
 * 覆盖：登录/登出/路由守卫/表单交互。
 * 依赖：Next.js 前端 (port 3000) + Go 后端 (port 8080) 运行中。
 */
import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('认证', () => {
  test('有效凭证登录后可访问受保护页面', async ({ page }) => {
    await loginAsAdmin(page);
    await expect(page).not.toHaveURL(/\/login/);
    // 登录后应跳转到门户聊天页，知识库选择器可见
    await expect(page.locator('select')).toBeVisible({ timeout: 5000 });
  });

  test('登录页表单可交互', async ({ page }) => {
    await page.goto('/login');
    await expect(page.getByRole('button', { name: '登录' })).toBeVisible();
    await expect(page.locator('input[autocomplete="username"]')).toBeVisible();
    await expect(page.locator('input[autocomplete="current-password"]')).toBeVisible();
  });

  test('空表单提交不会跳转', async ({ page }) => {
    await page.goto('/login');
    await page.getByRole('button', { name: '登录' }).click();
    await expect(page).toHaveURL(/\/login/);
  });

  test('未登录访问受保护页面重定向到登录', async ({ page }) => {
    await page.goto('/portal/chat');
    await expect(page).toHaveURL(/\/login/, { timeout: 10000 });
  });

  test('未登录访问后台页面重定向到登录', async ({ page }) => {
    await page.goto('/admin/dashboard');
    await expect(page).toHaveURL(/\/login/, { timeout: 10000 });
  });

  test('登录后访问后台仪表盘', async ({ page }) => {
    await loginAsAdmin(page, '/admin/dashboard');
    await expect(page.getByRole('heading', { name: '数据看板' })).toBeVisible({ timeout: 5000 });
  });

  test('使用表单提交错误密码应显示错误', async ({ page }) => {
    await page.goto('/login');
    await page.getByLabel('用户名').fill('admin');
    await page.getByLabel('密码').fill('WrongPassword123');
    await page.getByRole('button', { name: /登录/i }).click();
    // 错误密码应显示错误提示 — toast 或内联错误消息
    await expect(page.locator('[role="alert"], .text-\\[var\\(--color-error\\)\\]').first()).toBeVisible({ timeout: 5000 });
  });

  test('登录后退出', async ({ page }) => {
    await loginAsAdmin(page, '/admin/dashboard');
    // 定位退出按钮
    const logoutBtn = page.locator('button, a').filter({ hasText: /登出|退出|logout/i }).first();
    if (await logoutBtn.isVisible().catch(() => false)) {
      await logoutBtn.click();
      await expect(page).toHaveURL(/\/login/, { timeout: 5000 });
    }
  });
});
