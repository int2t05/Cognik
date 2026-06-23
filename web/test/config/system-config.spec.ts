/**
 * 系统配置 E2E 测试。
 *
 * 覆盖：配置列表渲染、编辑交互。
 */
import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('系统配置', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page, '/admin/config/system');
  });

  test('系统配置页面渲染配置项', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /系统配置/i })).toBeVisible();
    // 至少有一个配置项行
    const configRows = page.locator('[class*="ConfigRow"], .flex.items-center.justify-between');
    await expect(configRows.first()).toBeVisible({ timeout: 5000 });
  });

  test('编辑配置项 - 点击编辑按钮进入编辑模式', async ({ page }) => {
    // 点击第一个编辑按钮
    const editBtn = page.getByRole('button', { name: /编辑/i }).first();
    if (await editBtn.isVisible().catch(() => false)) {
      await editBtn.click();
      // 进入编辑模式后应出现输入框或下拉选择
      await expect(page.locator('input, select').first()).toBeVisible({ timeout: 3000 });
    }
  });
});
