/**
 * E2E 测试共享辅助函数。
 *
 * AuthProvider 从 localStorage 读取 auth state。proxy.ts 从 cookie 读取 JWT。
 * 两者需同时设置：cookie 供 middleware RBAC + localStorage 供 AuthProvider。
 */
import { expect, type Page } from '@playwright/test';

const API_URL = 'http://127.0.0.1:8080';
const CREDS = { username: 'admin', password: 'Admin@123' };

/**
 * 以管理员身份登录并导航到目标页面。
 *
 * 1. 通过 API 获取 JWT token
 * 2. 设置 cookie（供 proxy.ts 校验 RBAC）
 * 3. 预写 localStorage（供 AuthProvider 初始化）
 * 4. 导航到目标页并验证中间件未拦截
 */
export async function loginAsAdmin(page: Page, targetPath = '/portal/chat') {
  const res = await page.request.post(`${API_URL}/api/v1/auth/login`, {
    data: CREDS,
    headers: { 'Content-Type': 'application/json' },
  });
  expect(res.status()).toBe(200);
  const json = await res.json();
  expect(json.code).toBe(0);
  const data = json.data;
  const token: string = data.access_token;
  const refreshToken: string = data.refresh_token;

  // 1. Cookie — 供 proxy.ts JWT 校验
  await page.context().addCookies([
    { name: 'access_token', value: token, path: '/', domain: '127.0.0.1' },
    { name: 'refresh_token', value: refreshToken, path: '/', domain: '127.0.0.1' },
  ]);

  // 2. localStorage — addInitScript 在页面 JS 前执行
  const authJson = JSON.stringify({
    token,
    refreshToken,
    user: data.user,
    roles: data.roles,
    permissions: data.permissions,
    menus: data.menus,
    isLoggedIn: true,
  });
  await page.context().addInitScript((s) => {
    window.localStorage.setItem('auth', s);
  }, authJson);

  // 3. 导航 — _tokenGetter 直接从 localStorage 读取，无需等待 AuthProvider
  await page.goto(targetPath, { waitUntil: 'networkidle' });
  await expect(page).not.toHaveURL(/\/login/, { timeout: 10000 });
}
