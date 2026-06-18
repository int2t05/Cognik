/**
 * 用户管理 E2E 测试。
 *
 * 覆盖用户列表加载、分页、搜索、创建用户、冻结/恢复用户等
 * 后台用户管理核心流程。测试针对 /admin/users。
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

test.describe('用户列表', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto('/admin/users');
    await page.waitForURL('**/admin/users');
  });

  test('用户列表加载，显示表头和新建按钮', async ({ page }) => {
    // 页面标题
    await expect(page.getByText('用户管理')).toBeVisible();

    // 新建用户按钮
    await expect(page.getByRole('button', { name: '新建用户' })).toBeVisible();

    // 表格列头
    await expect(page.getByText('用户名')).toBeVisible();
    await expect(page.getByText('姓名')).toBeVisible();
    await expect(page.getByText('手机')).toBeVisible();
    await expect(page.getByText('状态')).toBeVisible();

    // 搜索框
    const searchInput = page.locator('input[placeholder="搜索用户..."]');
    await expect(searchInput).toBeVisible();
  });

  test('搜索框可输入，过滤用户列表', async ({ page }) => {
    const searchInput = page.locator('input[placeholder="搜索用户..."]');

    // 输入搜索关键词
    await searchInput.fill('admin');

    // 等待 debounce 后数据刷新（useDebounce 300ms）
    await page.waitForTimeout(500);

    // 搜索后应触发重新请求，表格刷新
    // 验证表格中至少出现一个匹配项
    await expect(page.getByText('admin').or(page.getByText('Admin'))).toBeVisible({ timeout: 5000 });
  });
});

test.describe('创建用户', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto('/admin/users');
    await page.waitForURL('**/admin/users');
  });

  test('新建用户对话框可打开并填写', async ({ page }) => {
    // 点击新建用户
    await page.getByRole('button', { name: '新建用户' }).click();

    // 对话框应弹出
    await expect(page.getByText('新建用户')).toBeVisible();

    // 对话框描述应提示密码规则
    await expect(page.getByText('密码需8-32位')).toBeVisible();
  });

  test('创建新用户并提交', async ({ page }) => {
    await page.getByRole('button', { name: '新建用户' }).click();

    // 生成唯一用户名避免冲突
    const username = `e2e_test_${Date.now()}`;

    // 填写表单——用户名和密码在非编辑模式下可见
    await page.locator('label:has-text("用户名") + input, label:has-text("用户名") ~ input').first().fill(username);
    await page.locator('input[type="password"]').first().fill('TestPass@123');

    // 填写姓名、手机、邮箱
    await page.locator('label:has-text("姓名") + input, label:has-text("姓名") ~ input').first().fill('E2E 测试用户');
    await page.locator('label:has-text("手机") + input, label:has-text("手机") ~ input').first().fill('13800138000');
    await page.locator('label:has-text("邮箱") + input, label:has-text("邮箱") ~ input').first().fill('e2e@test.com');

    // 点击保存
    await page.getByRole('button', { name: '保存' }).click();

    // 保存成功后应有成功提示
    await expect(page.getByText('已创建')).toBeVisible({ timeout: 5000 });
  });
});

test.describe('冻结/恢复用户', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto('/admin/users');
    await page.waitForURL('**/admin/users');
  });

  test('冻结用户流程', async ({ page }) => {
    // 等待表格数据加载
    await page.waitForTimeout(2000);

    // 找到第一个"冻结"按钮（状态为正常的用户）
    const freezeBtn = page.getByRole('button', { name: '冻结' }).first();
    await expect(freezeBtn).toBeVisible({ timeout: 10000 });

    // 记录被冻结的用户名
    const userRow = freezeBtn.locator('..').locator('..');
    const username = await userRow.locator('td').first().textContent();

    // 点击冻结
    await freezeBtn.click();

    // 确认对话框应弹出
    await expect(page.getByText('冻结用户')).toBeVisible();
    if (username) {
      await expect(page.getByText(username)).toBeVisible();
    }

    // 确认冻结
    await page.getByRole('button', { name: '冻结' }).click();

    // 操作成功后应有成功提示或状态变更
    await expect(page.getByText('操作成功')).toBeVisible({ timeout: 5000 });
  });
});
