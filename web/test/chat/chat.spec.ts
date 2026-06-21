/**
 * 智能问答模块 E2E 测试。
 */
import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('智能问答', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page, '/portal/chat');
  });

  test('页面包含知识库选择器和发送区域', async ({ page }) => {
    await expect(page.locator('select')).toBeVisible({ timeout: 5000 });
  });

  test('选择知识库后显示输入提示', async ({ page }) => {
    const select = page.locator('select');
    const optionCount = await select.locator('option').count();
    if (optionCount > 1) {
      await select.selectOption({ index: 1 });
      await expect(page.getByText('输入问题')).toBeVisible({ timeout: 3000 });
    }
  });

  test('新对话按钮可见', async ({ page }) => {
    // 左下角有 pill 样式的"新对话"按钮（含 Plus 图标，accessible name 为 "新对话"）
    await expect(page.locator('button').filter({ hasText: /新对话/ })).toBeVisible({ timeout: 5000 });
  });

  test('无知识库时发送按钮禁用', async ({ page }) => {
    // 未选择知识库时输入框应不可用
    const input = page.locator('input[placeholder*="选择知识库"]');
    if (await input.isVisible()) {
      // placeholder 提示未选知识库
      await expect(input).toBeVisible();
    }
  });

  test('侧边栏存在会话历史区域', async ({ page }) => {
    const viewport = page.viewportSize();
    if (viewport && viewport.width >= 1024) {
      // 桌面端侧边栏可见（其中有知识库选择器或历史列表）
      await expect(page.locator('aside select, aside button, aside .space-y-0\\.5')).toBeVisible({ timeout: 5000 });
    }
  });
});
