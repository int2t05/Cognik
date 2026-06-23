/**
 * E2E 测试共享辅助函数。
 *
 * AuthProvider 从 localStorage 读取 auth state。proxy.ts 从 cookie 读取 JWT。
 * 两者需同时设置：cookie 供 middleware RBAC + localStorage 供 AuthProvider。
 *
 * API_URL 可通过 PLAYWRIGHT_API_URL 环境变量覆盖，适配不同测试环境。
 * waitUntil: 'domcontentloaded' 比 'networkidle' 更可靠、更快，避免等待非关键资源。
 */
import { expect, type Page } from '@playwright/test';

/** API 基础地址，可通过 PLAYWRIGHT_API_URL 环境变量覆盖 */
export const API_URL = process.env.PLAYWRIGHT_API_URL || 'http://127.0.0.1:8080';

/** 管理员账号凭据 */
export const ADMIN_CREDS = { username: 'admin', password: 'Admin@123' };

/** 门户用户（报障人）凭据 */
export const PORTAL_USER_CREDS = { username: 'zhangsan', password: 'User@123' };

/**
 * 使用指定凭据登录并导航到目标页面。
 *
 * 1. 通过 API 获取 JWT token
 * 2. 设置 cookie（供 proxy.ts 校验 RBAC）
 * 3. 预写 localStorage（供 AuthProvider 初始化）
 * 4. 导航到目标页并验证中间件未拦截
 *
 * 提取为共享函数以避免 loginAsAdmin/loginAsPortalUser 的代码重复。
 */
async function loginWithCreds(page: Page, creds: { username: string; password: string }, targetPath: string) {
  const res = await page.request.post(`${API_URL}/api/v1/auth/login`, {
    data: creds,
    headers: { 'Content-Type': 'application/json' },
  });
  expect(res.status()).toBe(200);
  const json = await res.json();
  expect(json.code).toBe(0);
  const data = json.data;
  const token: string = data.access_token;
  const refreshToken: string = data.refresh_token;

  // 从 API_URL 解析域名用于 cookie（支持 localhost 和 127.0.0.1）
  const cookieDomain = new URL(API_URL).hostname;

  // 1. Cookie — 供 proxy.ts JWT 校验
  await page.context().addCookies([
    { name: 'access_token', value: token, path: '/', domain: cookieDomain },
    { name: 'refresh_token', value: refreshToken, path: '/', domain: cookieDomain },
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

  // 3. 导航 — domcontentloaded 比 networkidle 更可靠、更快
  await page.goto(targetPath, { waitUntil: 'domcontentloaded' });
  await expect(page).not.toHaveURL(/\/login/, { timeout: 10000 });
}

/**
 * 以管理员身份登录并导航到目标页面。
 */
export async function loginAsAdmin(page: Page, targetPath = '/portal/chat') {
  await loginWithCreds(page, ADMIN_CREDS, targetPath);
}

/**
 * 以门户用户（报障人）身份登录并导航到目标页面。
 *
 * 门户用户仅有前台权限，无后台管理权限，用于测试角色隔离。
 */
export async function loginAsPortalUser(page: Page, targetPath = '/portal/chat') {
  await loginWithCreds(page, PORTAL_USER_CREDS, targetPath);
}

/**
 * 退出当前用户 — 清除浏览器状态并跳转到登录页。
 */
export async function logoutUser(page: Page) {
  await page.context().clearCookies();
  await page.evaluate(() => {
    window.localStorage.clear();
    window.sessionStorage.clear();
  });
  await page.goto('/login', { waitUntil: 'domcontentloaded' });
}

/**
 * 调用后端清理测试产生的数据。
 *
 * 约定：所有 E2E 测试创建的数据都应包含 "e2e" 或 "E2E" 前缀，
 * 后端提供专用清理端点用于测试环境。非测试环境可能没有此端点，静默失败是预期行为。
 */
export async function resetTestData(page: Page) {
  try {
    await page.request.post(`${API_URL}/api/v1/test/e2e-cleanup`, {
      headers: { 'Content-Type': 'application/json' },
    });
  } catch {
    // 非测试环境可能没有清理端点，静默跳过
  }
}
