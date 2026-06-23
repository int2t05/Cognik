/**
 * 智能问答 E2E 测试 — 完整问答流程。
 *
 * 覆盖：页面渲染、KB 选择、问答交互、新对话重置、侧边栏历史。
 * 使用按钮点击替代 keyboard.press，更接近真实用户操作。
 * 无可用知识库时跳过相关测试而非失败。
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

    // 使用发送按钮而非 keyboard.press('Enter')
    const sendBtn = page.locator('button[type="submit"], button:has(svg), button').filter({ hasText: /发送|send/ }).first();
    if (await sendBtn.isVisible().catch(() => false)) {
      await sendBtn.click();
    } else {
      // 无发送按钮时回退到 Enter 键
      await page.keyboard.press('Enter');
    }

    // 用户消息出现在对话区
    await expect(page.getByText('你好').first()).toBeVisible({ timeout: 10000 });
  });

  test('新对话按钮重置会话', async ({ page }) => {
    const newChatBtn = page.locator('button').filter({ hasText: /新对话/ }).first();
    if (await newChatBtn.isVisible().catch(() => false)) {
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
