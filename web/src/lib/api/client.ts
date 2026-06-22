/** API 客户端 — fetch 封装 + 统一错误处理 + 类型安全响应解包。 */

import type { ApiResponse, PageResponse } from './types';

// 开发模式直接连后端（绕过 Next.js rewrite，避免 Turbopack POST 代理 500）
// 生产模式通过 NEXT_PUBLIC_API_URL 配置
const BASE_URL = process.env.NEXT_PUBLIC_API_URL || (
  typeof window !== 'undefined' ? 'http://localhost:8080' : ''
);

// 模块级 token getter，默认从 localStorage 直接读取。
// AuthProvider 仍可通过 setTokenGetter 覆盖（如 login/logout 后立即生效），
// 但默认行为不依赖 AuthProvider 调用时序，避免 HMR 模块重置导致丢失。
let _tokenGetter: () => string | null = () => {
  try {
    const stored = localStorage.getItem('auth');
    if (stored) return JSON.parse(stored).token || null;
  } catch { /* SSR / permission denied */ }
  return null;
};

/**
 * setTokenGetter 设置用于自动附加 Authorization header 的 token getter。
 * 在 AuthProvider 初始化时调用，传入从 auth state 读取 token 的函数。
 */
export function setTokenGetter(getter: () => string | null) {
  _tokenGetter = getter;
}

export class ApiError extends Error {
  constructor(
    public code: number,
    message: string
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

/**
 * 安全解析 JSON 响应体。
 * 处理空响应体、非 JSON 响应（如 HTML 错误页）、JSON 解析失败等情况。
 */
async function safeResJson(res: Response): Promise<unknown> {
  // 先尝试读取文本，避免 res.json() 在空响应体上抛错
  const text = await res.text();
  if (!text) {
    throw new ApiError(
      res.status,
      res.status === 502 || res.status === 503
        ? '后端服务不可达，请确认服务已启动（端口 8080）'
        : `服务器返回空响应 (HTTP ${res.status})`
    );
  }
  try {
    return JSON.parse(text);
  } catch {
    // HTML 错误页或纯文本错误
    const preview = text.slice(0, 200).replace(/\n/g, ' ');
    throw new ApiError(res.status, `服务器返回非 JSON 响应 (HTTP ${res.status}): ${preview}`);
  }
}

/**
 * rawApiRequest 发送已认证的 API 请求，返回完整响应 JSON。
 * apiFetch 和 apiFetchPage 共享此底层调用，消除重复的请求构造逻辑。
 */
async function rawApiRequest(url: string, options?: RequestInit): Promise<Record<string, unknown>> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options?.headers as Record<string, string>),
  };
  const token = _tokenGetter();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  let res: Response;
  try {
    res = await fetch(`${BASE_URL}${url}`, { ...options, headers });
  } catch (err) {
    throw new Error(
      err instanceof TypeError && err.message === 'Failed to fetch'
        ? '后端服务不可达，请确认服务已启动（端口 8080）'
        : err instanceof Error ? err.message : '网络请求失败'
    );
  }

  const json = (await safeResJson(res)) as Record<string, unknown>;

  if (json.code !== 0) {
    throw new ApiError(json.code as number, json.message as string || `请求失败 (code=${json.code})`);
  }

  return json;
}

/** 通用 JSON API 调用，返回 data 字段 */
export async function apiFetch<T>(url: string, options?: RequestInit): Promise<T> {
  const json = await rawApiRequest(url, options);
  return json.data as T;
}

/** 分页 API 调用，返回类型安全的 PageResponse */
export async function apiFetchPage<T>(url: string): Promise<PageResponse<T>> {
  const json = await rawApiRequest(url);
  return {
    items: json.data as T[],
    total: json.total as number,
    page: json.page as number,
    pageSize: json.page_size as number,
  };
}

/** SWR 默认 fetcher */
// TODO: swrFetcher 未被任何文件引用，考虑移除或统一 SWR fetcher 模式
export const swrFetcher = (url: string) => apiFetch(url);
