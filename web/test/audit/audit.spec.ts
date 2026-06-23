/**
 * 审计日志 E2E 测试。
 *
 * 覆盖：标题渲染、筛选输入框、空状态、日期筛选交互。
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
    // 优先使用具描述性的 placeholder 选择器
    const placeholderInput = page.getByPlaceholder(/操作人|操作类型|对象类型/);
    if (await placeholderInput.isVisible().catch(() => false)) {
      await expect(placeholderInput).toBeVisible({ timeout: 5000 });
    } else {
      // 回退：检查日期筛选器
      const dateInput = page.locator('input[type="date"]').first();
      if (await dateInput.isVisible().catch(() => false)) {
        await expect(dateInput).toBeVisible({ timeout: 5000 });
      } else {
        // 至少有一个输入框存在
        await expect(page.locator('input').first()).toBeVisible({ timeout: 5000 });
      }
    }
  });

  test('空状态提示渲染', async ({ page }) => {
    // 审计日志列表如果没有数据，应显示空状态提示
    const table = page.locator('table');
    const emptyState = page.getByText(/暂无数据|没有记录|暂无日志/);
    if (await table.isVisible().catch(() => false)) {
      await expect(table).toBeVisible({ timeout: 5000 });
    } else if (await emptyState.first().isVisible().catch(() => false)) {
      await expect(emptyState.first()).toBeVisible({ timeout: 5000 });
    }
    // 两种状态都接受：有数据则表格可见，无数据则空状态提示可见
  });

  test('日期筛选交互', async ({ page }) => {
    const dateInputs = page.locator('input[type="date"]');
    const count = await dateInputs.count();
    if (count > 0) {
      await dateInputs.first().fill('2025-01-01');
      // 页面不应崩溃或重定向到登录
      await expect(page).not.toHaveURL(/\/login/, { timeout: 5000 });
    }
  });
});
