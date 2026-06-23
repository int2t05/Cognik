/**
 * API 集成测试共享辅助函数。
 *
 * 使用 Playwright 的 request fixture 直接调用后端 API，无需浏览器。
 * 每个测试创建独立的资源，完成后清理，保证可重复执行。
 */
import { expect, type APIRequestContext, type APIResponse } from '@playwright/test';

const API_URL = 'http://127.0.0.1:8080';
const ADMIN = { username: 'admin', password: 'Admin@123' };
const TEST_USER = { username: 'e2e_test_user', password: 'Test@123!' };

export interface AuthTokens {
  accessToken: string;
  refreshToken: string;
  user: { id: number; username: string };
  roles: string[];
  permissions: string[];
}

/** 以管理员身份登录，返回 token 信息 */
export async function loginAsAdmin(request: APIRequestContext): Promise<AuthTokens> {
  return login(request, ADMIN.username, ADMIN.password);
}

/** 通用登录 */
export async function login(
  request: APIRequestContext,
  username: string,
  password: string,
): Promise<AuthTokens> {
  const res = await request.post(`${API_URL}/api/v1/auth/login`, {
    data: { username, password },
    headers: { 'Content-Type': 'application/json' },
  });
  expect(res.status()).toBe(200);
  const json = await res.json();
  expect(json.code).toBe(0);
  return {
    accessToken: json.data.access_token,
    refreshToken: json.data.refresh_token,
    user: json.data.user,
    roles: json.data.roles,
    permissions: json.data.permissions,
  };
}

/** 刷新令牌 */
export async function refreshToken(
  request: APIRequestContext,
  refreshTokenStr: string,
): Promise<AuthTokens> {
  const res = await request.post(`${API_URL}/api/v1/auth/refresh`, {
    data: { refresh_token: refreshTokenStr },
    headers: { 'Content-Type': 'application/json' },
  });
  expect(res.status()).toBe(200);
  const json = await res.json();
  expect(json.code).toBe(0);
  return {
    accessToken: json.data.access_token,
    refreshToken: json.data.refresh_token,
    user: json.data.user,
    roles: json.data.roles,
    permissions: json.data.permissions,
  };
}

/** 创建带 auth header 的通用 headers（适用于 POST/PUT/PATCH，含 Content-Type） */
export function authHeaders(token: string): Record<string, string> {
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  };
}

/** 创建仅含认证信息的 headers（适用于 GET/DELETE，不含 Content-Type） */
export function getAuthHeaders(token: string): Record<string, string> {
  return {
    Authorization: `Bearer ${token}`,
  };
}

/** 构造完整的 API URL */
export function getApiUrl(path: string): string {
  return `${API_URL}${path}`;
}

/** 验证 API 成功响应。Go 后端统一返回 HTTP 200 */
export async function assertSuccess(res: APIResponse) {
  expect(res.status()).toBe(200);
  const json = await res.json();
  expect(json.code).toBe(0);
  return json;
}

/**
 * 验证 API 错误响应。
 * Go 后端：参数校验失败 → HTTP 400，权限认证失败 → 401/403，资源不存在 → 404，冲突 → 409
 */
export async function assertError(
  res: APIResponse,
  expectedCode: number,
  expectedStatus: number = 400,
) {
  expect(res.status()).toBe(expectedStatus);
  const json = await res.json();
  expect(json.code).toBe(expectedCode);
  return json;
}

/** 清理测试创建的资源 */
export async function cleanupEntity(
  request: APIRequestContext,
  token: string,
  url: string,
): Promise<boolean> {
  try {
    const res = await request.delete(url, { headers: getAuthHeaders(token) });
    return res.ok();
  } catch {
    return false;
  }
}

// 计数器 + 随机后缀确保跨 worker 唯一（每个 worker 有独立模块状态，random 避免碰撞）
let _counter = 0;
export function uniqueName(prefix: string): string {
  const rnd = Math.random().toString(36).slice(2, 6);
  return `${prefix}_${++_counter}_${rnd}`;
}

/** 生成唯一的 11 位手机号 */
let _phoneCounter = 90000;
export function uniquePhone(): string {
  const rnd = String(Math.random()).slice(2, 4);
  return `138${String(++_phoneCounter).padStart(6, '0')}${rnd}`;
}

export { API_URL, TEST_USER };
