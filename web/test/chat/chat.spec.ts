import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('智能问答', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page, '/portal/chat');
  });

  test('知识库选择器可见', async ({ page }) => {
    await expect(page.locator('select')).toBeVisible();
  });

  test('选择知识库更新 placeholder', async ({ page }) => {
    const select = page.locator('select');
    const count = await select.locator('option').count();
    if (count > 1) {
      await select.selectOption({ index: 1 });
      await expect(page.locator('text=输入问题')).toBeVisible({ timeout: 3000 });
    }
  });
});
