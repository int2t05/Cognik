/**
 * 角色管理 E2E 测试。
 *
 * 覆盖：标题渲染、表格渲染、创建角色弹窗交互。
 */
import { test, expect, request as pwRequest } from '@playwright/test';
import { loginAsAdmin, API_URL, ADMIN_CREDS } from '../helpers';

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

  test('创建角色弹窗交互', async ({ page }) => {
    const createBtn = page.getByRole('button', { name: /新建角色/i });
    if (await createBtn.isVisible().catch(() => false)) {
      await createBtn.click();
      // 等待 dialog 出现（Radix Portal 渲染）
      const dialog = page.locator('[role="dialog"]').first();
      await expect(dialog).toBeVisible({ timeout: 3000 });
      // 填写角色名
      const nameInput = dialog.getByLabel(/角色名/i);
      if (await nameInput.isVisible().catch(() => false)) {
        await nameInput.fill('e2e_test_role');
        const saveBtn = dialog.getByRole('button', { name: /保存/i });
        if (await saveBtn.isVisible().catch(() => false)) {
          await saveBtn.click();
        }
      }
    }
  });

  // 清理：删除 E2E 测试过程中创建的角色
  test.afterAll(async () => {
    const ctx = await pwRequest.newContext();
    try {
      const loginRes = await ctx.post(`${API_URL}/api/v1/auth/login`, {
        data: ADMIN_CREDS,
        headers: { 'Content-Type': 'application/json' },
      });
      if (loginRes.ok()) {
        // 调用后端清理端点，约定 e2e 测试数据前缀
        await ctx.post(`${API_URL}/api/v1/test/e2e-cleanup`).catch(() => {});
      }
    } finally {
      await ctx.dispose();
    }
  });
});
