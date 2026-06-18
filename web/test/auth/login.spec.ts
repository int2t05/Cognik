/**
 * 认证模块 E2E 测试。
 *
 * 覆盖登录流程、密码修改、权限跳转等核心认证场景。
 * 测试针对 OpsMind 门户系统，管理员角色登录后跳转 /admin/dashboard，
 * 普通用户角色跳转 /portal/chat。
 */

import { test, expect, type Page } from '@playwright/test';

/** 登录辅助函数 */
async function login(page: Page, username = 'admin', password = 'Admin@123') {
  await page.goto('/login');
  await page.locator('input[autocomplete="username"]').fill(username);
  await page.locator('input[autocomplete="current-password"]').fill(password);
  await page.getByRole('button', { name: '登录' }).click();
}

test.describe('登录页', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
  });

  test('使用有效凭证登录，管理员跳转至后台看板', async ({ page }) => {
    await login(page);
    // 管理员角色应跳转至 /admin/dashboard
    await page.waitForURL('**/admin/dashboard');
    await expect(page.getByText('运维数字员工系统')).toBeVisible();
  });

  test('使用无效凭证登录，显示错误提示', async ({ page }) => {
    await login(page, 'admin', 'wrong_password');
    // 页面上应出现错误提示 (toast 或内联错误)
    await expect(page.locator('text=登录失败').first()).toBeVisible({ timeout: 5000 });
    // 且 URL 仍停留在 /login
    await expect(page).toHaveURL(/\/login/);
  });

  test('空表单提交显示验证提示', async ({ page }) => {
    await page.getByRole('button', { name: '登录' }).click();
    // 前端校验：空用户名或密码时触发 toast.error('请输入用户名和密码')
    await expect(page.getByText('请输入用户名和密码')).toBeVisible({ timeout: 3000 });
  });

  test('密码修改流程', async ({ page }) => {
    // 管理员登录
    await login(page);

    // 等待跳转至后台
    await page.waitForURL('**/admin/dashboard');

    // 导航至修改密码页（通过侧边栏或直接 URL）
    await page.goto('/change-password');
    await page.waitForURL('**/change-password');

    // 填写旧密码和新密码
    const oldPasswordInputs = page.locator('input[autocomplete="current-password"]');
    const newPasswordInputs = page.locator('input[autocomplete="new-password"]');

    // change-password 页有三个密码输入框：旧密码、新密码、确认新密码
    await oldPasswordInputs.first().fill('Admin@123');
    await newPasswordInputs.first().fill('NewPass@123');
    await newPasswordInputs.last().fill('NewPass@123');

    // 提交
    await page.getByRole('button', { name: '修改密码' }).click();

    // 修改成功后应有成功提示
    await expect(page.getByText('密码修改成功')).toBeVisible({ timeout: 5000 });
  });
});
