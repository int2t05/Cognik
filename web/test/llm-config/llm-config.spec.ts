/**
 * LLM 配置 E2E 测试。
 */
import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('LLM 配置', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page, '/admin/config/llm');
  });

  test('显示标题', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'LLM 配置' })).toBeVisible({ timeout: 5000 });
  });

  test('页面内容正常渲染', async ({ page }) => {
    // 至少有一个交互元素
    await expect(page.locator('main button, main table, main form').first()).toBeVisible({ timeout: 5000 });
  });
});
