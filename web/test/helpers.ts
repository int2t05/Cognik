/**
 * E2E 测试共享辅助函数。
 *
 * AuthProvider 从 localStorage 读取 auth state，而非 cookie。
 * 使用 addInitScript 在 JS 执行前预写入 localStorage，避免导航到 /login。
 */
import { expect, type Page } from '@playwright/test';

const API_URL = 'http://127.0.0.1:8080';
const CREDS = { username: 'admin', password: 'Admin@123' };

export async function loginAsAdmin(page: Page, targetPath = '/portal/chat') {
  // 通过 API 获取凭据
  const res = await page.request.post(`${API_URL}/api/v1/auth/login`, {
    data: CREDS,
    headers: { 'Content-Type': 'application/json' },
  });
  const json = await res.json();
  const data = json.data;
  const token: string = data.access_token;
  const refreshToken: string = data.refresh_token;

  // 1. 写 cookie — 供 Next.js proxy 中间件 RBAC 检查
  await page.context().addCookies([
    { name: 'access_token', value: token, path: '/', domain: '127.0.0.1' },
    { name: 'refresh_token', value: refreshToken, path: '/', domain: '127.0.0.1' },
  ]);

  // 2. 预写 localStorage — 在页面 JS 执行前注入，AuthProvider 初始化时就能读到
  const authState = JSON.stringify({
    token,
    refreshToken,
    user: data.user,
    roles: data.roles,
    permissions: data.permissions,
    menus: data.menus,
    isLoggedIn: true,
  });
  await page.context().addInitScript((s) => {
    localStorage.setItem('auth', s);
  }, authState);

  // 3. 导航到目标页
  await page.goto(targetPath, { waitUntil: 'networkidle' });
  await expect(page).not.toHaveURL(/\/login/, { timeout: 5000 });
}
