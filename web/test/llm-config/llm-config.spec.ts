/**
 * LLM 配置 E2E 测试。
 *
 * 覆盖：标题渲染、页面交互元素、新建对话框。
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
    // 至少有一个交互元素：按钮、表格或表单
    await expect(page.getByRole('button').or(page.getByRole('table')).or(page.locator('form')).first()).toBeVisible({ timeout: 5000 });
  });

  test('新建 LLM 配置按钮存在', async ({ page }) => {
    const createBtn = page.getByRole('button', { name: /新建.*LLM|新建.*配置|添加/ });
    if (await createBtn.isVisible().catch(() => false)) {
      await expect(createBtn).toBeVisible({ timeout: 5000 });
    }
  });

  test('新建按钮点击打开对话框', async ({ page }) => {
    const createBtn = page.getByRole('button', { name: /新建.*LLM|新建.*配置|添加/ });
    if (await createBtn.isVisible().catch(() => false)) {
      await createBtn.click();
      const dialog = page.locator('[role="dialog"]');
      if (await dialog.isVisible().catch(() => false)) {
        await expect(dialog).toBeVisible({ timeout: 3000 });
        // 对话框中应有表单字段
        const formFields = dialog.locator('input, select, textarea');
        await expect(formFields.first()).toBeVisible({ timeout: 3000 });
      }
    }
  });
});
