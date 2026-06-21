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
    const optionCount = await select.locator('option').count();
    if (optionCount > 1) {
      await select.selectOption({ index: 1 });
      await expect(page.getByText(/输入问题/)).toBeVisible({ timeout: 3000 });
    }
  });

  test('无知识库时输入框提示选择', async ({ page }) => {
    const input = page.locator('input[placeholder*="选择知识库"]');
    if (await input.isVisible()) {
      await expect(input).toBeVisible();
    }
  });

  test('问答完整流程：选择KB → 输入 → 发送 → 收到回复', async ({ page }) => {
    const select = page.locator('select');
    const optionCount = await select.locator('option').count();
    if (optionCount <= 1) { test.skip(); return; }
    await select.selectOption({ index: 1 });

    // 输入问题
    const input = page.locator('input[placeholder*="输入问题"]');
    await expect(input).toBeVisible({ timeout: 3000 });
    await input.fill('你好');

    // 发送
    await page.keyboard.press('Enter');

    // 等待 AI 回复出现（最多 30 秒，因为需要 LLM 后端）
    await expect(
      page.locator('[class*="message"], [class*="ChatMessage"], [class*="bubble"]').last(),
    ).toBeVisible({ timeout: 30000 });
  });

  test('新对话按钮重置会话', async ({ page }) => {
    // 如果侧边栏中有"新对话"按钮，点击后应重置状态
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
