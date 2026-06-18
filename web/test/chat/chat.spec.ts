/**
 * 智能问答（Chat）E2E 测试。
 *
 * 覆盖知识库选择、消息发送、会话管理等核心问答交互流程。
 * 注意：AI 响应本身在 E2E 测试中不做断言（依赖外部 LLM 服务），
 * 仅验证 UI 交互层面（输入框、消息气泡、按钮状态等）。
 */

import { test, expect, type Page } from '@playwright/test';

/** 管理员登录辅助函数 */
async function loginAsAdmin(page: Page) {
  await page.goto('/login');
  await page.locator('input[autocomplete="username"]').fill('admin');
  await page.locator('input[autocomplete="current-password"]').fill('Admin@123');
  await page.getByRole('button', { name: '登录' }).click();
  await page.waitForURL('**/admin/dashboard');
}

/** 导航至门户聊天页（管理员可通过切换角色或在浏览器直接访问 /portal/chat） */
async function navigateToChat(page: Page) {
  await page.goto('/portal/chat');
  await page.waitForURL('**/portal/chat');
}

test.describe('智能问答', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
    await navigateToChat(page);
  });

  test('进入聊天页，知识库选择器可见', async ({ page }) => {
    // 知识库选择器 select 元素
    const kbSelect = page.locator('select').first();
    await expect(kbSelect).toBeVisible();

    // 默认显示"选择知识库..."占位选项
    await expect(kbSelect.locator('option[value="0"]')).toHaveText('选择知识库...');
  });

  test('选择一个知识库后，输入框 placeholder 更新', async ({ page }) => {
    const kbSelect = page.locator('select').first();

    // 等待知识库列表加载（需要 API 返回数据）
    // 选择第一个有值的知识库（非占位项）
    const options = kbSelect.locator('option');
    const count = await options.count();
    if (count <= 1) {
      test.skip(count <= 1, '无可用知识库，跳过测试');
      return;
    }

    // 选择第一个实际知识库
    const firstKBValue = await options.nth(1).getAttribute('value');
    await kbSelect.selectOption(firstKBValue!);

    // 发送按钮或输入框应处于可用状态（不再 disabled）
    // ChatInput 组件在 selectedKB 非 0 时启用
    const chatInput = page.locator('input[placeholder*="输入问题"]').first();
    await expect(chatInput).toBeEnabled({ timeout: 3000 });
  });

  test('发送一条消息，用户消息气泡出现', async ({ page }) => {
    const kbSelect = page.locator('select').first();
    const options = kbSelect.locator('option');
    const count = await options.count();
    if (count <= 1) {
      test.skip(count <= 1, '无可用知识库，跳过测试');
      return;
    }

    // 选择知识库
    const firstKBValue = await options.nth(1).getAttribute('value');
    await kbSelect.selectOption(firstKBValue!);

    // 输入消息
    const chatInput = page.locator('input[placeholder*="输入问题"]').first();
    await expect(chatInput).toBeEnabled({ timeout: 3000 });
    await chatInput.fill('测试消息：什么是 VPN？');

    // 按 Enter 发送
    await chatInput.press('Enter');

    // 用户消息气泡应出现在消息列表中
    // 消息气泡由 ChatMessage 组件渲染，role 为 'user'
    await expect(page.getByText('测试消息：什么是 VPN？')).toBeVisible({ timeout: 5000 });
  });

  test('点击"新对话"按钮，消息列表清空', async ({ page }) => {
    const kbSelect = page.locator('select').first();
    const options = kbSelect.locator('option');
    const count = await options.count();
    if (count <= 1) {
      test.skip(count <= 1, '无可用知识库，跳过测试');
      return;
    }

    // 选择知识库并发送消息
    const firstKBValue = await options.nth(1).getAttribute('value');
    await kbSelect.selectOption(firstKBValue!);

    const chatInput = page.locator('input[placeholder*="输入问题"]').first();
    await expect(chatInput).toBeEnabled({ timeout: 3000 });
    await chatInput.fill('临时消息');
    await chatInput.press('Enter');
    await page.waitForTimeout(1000);

    // 点击"新对话"按钮（sidebar 中的 AppleButton 或主区域的）
    const newChatBtn = page.getByRole('button', { name: '新对话' }).first();
    await newChatBtn.click();

    // 消息列表应清空，显示占位提示
    // 注意：历史会话侧边栏可能仍保留记录，但主消息区应显示初始占位
    await expect(page.getByText('请先选择一个知识库').or(page.getByText('输入问题开始对话'))).toBeVisible({ timeout: 3000 });
  });
});
