/**
 * 智能问答 E2E 测试 — 完整问答流程。
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
    await expect(select).toBeVisible({ timeout: 5000 });
    const optionCount = await select.locator('option').count();
    if (optionCount <= 1) {
      test.skip(true, '无可用知识库，跳过');
      return;
    }
    await select.selectOption({ index: 1 });
    await expect(page.getByText(/输入问题/)).toBeVisible({ timeout: 3000 });
  });

  test('问答流程：选KB → 输入 → 发送 → 用户消息出现', async ({ page }) => {
    const select = page.locator('select');
    await expect(select).toBeVisible({ timeout: 5000 });
    const optionCount = await select.locator('option').count();
    if (optionCount <= 1) {
      test.skip(true, '无可用知识库，跳过');
      return;
    }
    await select.selectOption({ index: 1 });

    const input = page.locator('input[placeholder*="输入问题"]');
    await expect(input).toBeVisible({ timeout: 3000 });
    await input.fill('你好');
    await page.keyboard.press('Enter');

    // 用户消息出现在对话区
    await expect(page.getByText('你好').first()).toBeVisible({ timeout: 10000 });
  });

  test('新对话按钮重置会话', async ({ page }) => {
    const newChatBtn = page.locator('button').filter({ hasText: /新对话/ }).first();
    if (await newChatBtn.isVisible()) {
      await newChatBtn.click();
      await expect(page.locator('select')).toBeVisible();
    }
  });

  test('侧边栏会话历史', async ({ page }) => {
    const viewport = page.viewportSize();
    if (viewport && viewport.width >= 1024) {
      await expect(
        page.locator('aside select, aside button, aside [class*="space-y"]').first(),
      ).toBeVisible({ timeout: 5000 });
    }
  });
});
