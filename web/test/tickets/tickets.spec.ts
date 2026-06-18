/**
 * 申告管理 E2E 测试。
 *
 * 覆盖门户端申告列表、新建申告表单、后台申告管理列表、
 * 以及后台申告详情处理操作等核心流程。
 * 测试覆盖门户（/portal/tickets）和后台（/admin/tickets）两端。
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

test.describe('门户端申告列表', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto('/portal/tickets');
    await page.waitForURL('**/portal/tickets');
  });

  test('我的申告页面加载，显示表头', async ({ page }) => {
    // 页面标题
    await expect(page.getByText('我的申告')).toBeVisible();
    // 表格列头
    await expect(page.getByText('编号')).toBeVisible();
    await expect(page.getByText('标题')).toBeVisible();
    await expect(page.getByText('状态')).toBeVisible();
  });
});

test.describe('新建申告', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto('/portal/tickets/new');
    await page.waitForURL('**/portal/tickets/new');
  });

  test('新建申告表单可见并可填写', async ({ page }) => {
    // 页面标题
    await expect(page.getByText('提交申告')).toBeVisible();

    // 填写表单
    await page.locator('input[placeholder="简要描述遇到的问题"]').fill('E2E 测试申告');
    await page.locator('textarea[placeholder*="详细描述问题现象"]').fill('这是一个 Playwright E2E 测试产生的申告。');

    // 选择紧急程度
    const urgencySelect = page.locator('select').first();
    await urgencySelect.selectOption('3');

    // 提交
    await page.getByRole('button', { name: '提交申告' }).click();

    // 提交成功后应跳转至列表页
    await page.waitForURL('**/portal/tickets', { timeout: 10000 });
  });

  test('空标题提交显示验证提示', async ({ page }) => {
    // 不填标题直接提交
    await page.getByRole('button', { name: '提交申告' }).click();

    // 应显示前端校验错误
    await expect(page.getByText('请输入申告标题')).toBeVisible({ timeout: 3000 });
  });
});

test.describe('后台申告管理', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto('/admin/tickets');
    await page.waitForURL('**/admin/tickets');
  });

  test('申告管理列表加载，显示筛选按钮', async ({ page }) => {
    // 页面标题
    await expect(page.getByText('申告管理')).toBeVisible();

    // 筛选按钮组：全部、待处理、处理中、需补充、已解决、已关闭
    await expect(page.getByText('全部')).toBeVisible();
    await expect(page.getByText('待处理')).toBeVisible();
    await expect(page.getByText('处理中')).toBeVisible();
    await expect(page.getByText('已解决')).toBeVisible();
    await expect(page.getByText('已关闭')).toBeVisible();

    // 表格列头
    await expect(page.getByText('编号')).toBeVisible();
    await expect(page.getByText('提交人')).toBeVisible();
  });

  test('点击筛选按钮切换状态', async ({ page }) => {
    // 点击"待处理"筛选
    await page.getByText('待处理').click();

    // 筛选按钮应高亮（选中状态类名变化）
    // 验证筛选按钮处于激活状态（背景色为 accent 蓝色）
    const activeFilter = page.locator('button').filter({ hasText: '待处理' });
    await expect(activeFilter).toHaveClass(/bg-\[var\(--color-accent\)\]/);
  });
});

test.describe('后台申告详情', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto('/admin/tickets');
    await page.waitForURL('**/admin/tickets');
  });

  test('点击申告标题跳转至详情页', async ({ page }) => {
    // 等待表格加载完成
    await page.waitForTimeout(2000);

    // 点击第一个申告的标题链接
    const firstLink = page.locator('table a').first();
    await expect(firstLink).toBeVisible({ timeout: 10000 });
    await firstLink.click();

    // 跳转至详情页
    await page.waitForURL(/\/admin\/tickets\/\d+/);

    // 详情页应显示申告标题、状态标签和操作按钮
    // 状态为 1（待处理）时应有"开始处理"按钮
    await expect(page.getByRole('button', { name: '开始处理' }).or(page.getByRole('button', { name: '标记解决' }).or(page.getByRole('button', { name: '关闭申告' })))).toBeVisible({ timeout: 5000 });
  });
});
