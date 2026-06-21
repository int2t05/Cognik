/**
 * 审计日志 E2E 测试。
 */
import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('审计日志', () => {
  test.beforeEach(async ({ page }) => {
    // 审计日志页面路由为 /admin/audit
    await loginAsAdmin(page, '/admin/audit');
  });

  test('显示标题', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '审计日志' })).toBeVisible({ timeout: 5000 });
  });

  test('筛选输入框存在', async ({ page }) => {
    // 日期筛选器 type="date"
    const inputs = page.locator('input');
    await expect(inputs.first()).toBeVisible({ timeout: 5000 });
  });
});
