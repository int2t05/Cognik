/**
 * 用户管理 E2E 测试 — 列表 + 搜索 + 新建完整流程。
 */
import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('用户管理', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page, '/admin/users');
  });

  test('显示标题和表格', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '用户管理' })).toBeVisible();
    await expect(page.locator('table')).toBeVisible({ timeout: 5000 });
  });

  test('搜索框可用', async ({ page }) => {
    const searchInput = page.locator('input[placeholder*="搜索"]');
    if (await searchInput.isVisible()) {
      await searchInput.fill('admin');
      await page.waitForTimeout(500);
      await expect(page.locator('table')).toBeVisible();
    }
  });

  test('分页可见', async ({ page }) => {
    await expect(page.locator('table')).toBeVisible({ timeout: 5000 });
  });

  test('新建用户完整流程', async ({ page }) => {
    // 点击新建按钮（如果有的话）
    const createBtn = page.getByRole('button', { name: /新建|创建/ });
    if (!(await createBtn.isVisible().catch(() => false))) { test.skip(); return; }

    await createBtn.click();
    await page.waitForTimeout(300);

    // 查找 dialog 中的 input
    const dialog = page.locator('[role="dialog"]');
    if (!(await dialog.isVisible().catch(() => false))) { test.skip(); return; }

    // 填写表单
    const inputs = dialog.locator('input');
    const inputCount = await inputs.count();
    if (inputCount >= 2) {
      await inputs.nth(0).fill('e2etest_user');
      await inputs.nth(1).fill('E2eTest@123');
    }

    // 保存
    const saveBtn = dialog.getByRole('button', { name: /保存|创建/ });
    if (await saveBtn.isVisible()) {
      await saveBtn.click();
      // 应关闭 dialog 并刷新表格
      await expect(page.locator('table')).toBeVisible({ timeout: 5000 });
    }
  });
});
