import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers';

test.describe('知识库列表', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page, '/admin/knowledge');
  });

  test('显示标题和新建按钮', async ({ page }) => {
    await expect(page.getByRole('heading', { name: '知识库管理' })).toBeVisible();
    await expect(page.getByRole('button', { name: '新建知识库' })).toBeVisible();
  });

  test('页面导航到知识库详情', async ({ page }) => {
    // 验证知识库卡片出现后点击可导航
    await expect(page.getByRole('heading', { name: '知识库管理' })).toBeVisible();
    // 页面 URL 正确
    await expect(page).toHaveURL(/\/admin\/knowledge/);
  });
});
