/** API 客户端 — fetch 封装 + 统一错误处理 + 类型安全响应解包。 */

import type { ApiResponse, PageResponse } from './types';

const BASE_URL = process.env.NEXT_PUBLIC_API_URL || '';

// 模块级 token getter，由 AuthProvider 注入。
// 使用函数引用而非直接存储值，保证 fetch 时拿到的总是最新 token。
let _tokenGetter: () => string | null = () => null;

/**
 * setTokenGetter 设置用于自动附加 Authorization header 的 token getter。
 * 在 AuthProvider 初始化时调用，传入从 auth state 读取 token 的函数。
 * 登录/登出/续期后 token 变化通过 getter 函数引用自动反映，无需额外调用。
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

/** 通用 JSON API 调用，返回 data 字段 */
export async function apiFetch<T>(
  url: string,
  options?: RequestInit
): Promise<T> {
  let res: Response;
  try {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options?.headers as Record<string, string>),
    };
    // 自动附加 Authorization header（login/refresh 等无需 token 的调用不受影响）
    const token = _tokenGetter();
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    res = await fetch(`${BASE_URL}${url}`, {
      ...options,
      headers,
    });
  } catch (err) {
    throw new Error(err instanceof Error ? err.message : 'Network error');
  }

  const json: ApiResponse<T> = await res.json();

  if (json.code !== 0) {
    throw new ApiError(json.code, json.message);
  }

  return json.data;
}

/** 分页 API 调用，返回类型安全的 PageResponse */
export async function apiFetchPage<T>(url: string): Promise<PageResponse<T>> {
  const headers: Record<string, string> = {};
  const token = _tokenGetter();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  const res = await fetch(`${BASE_URL}${url}`, { headers });
  const json = await res.json();

  if (json.code !== 0) {
    throw new ApiError(json.code, json.message);
  }

  return {
    items: json.data as T[],
    total: json.total as number,
    page: json.page as number,
    pageSize: json.page_size as number,
  };
}

/** SWR 默认 fetcher */
export const swrFetcher = (url: string) => apiFetch(url);
