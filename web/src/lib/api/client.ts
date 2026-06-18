/**
 * API 客户端 — fetch 封装 + 统一错误处理。
 *
 * 修复旧版响应解包不一致问题（res.data || res.items 等回退逻辑）。
 * 参照 docs/TODO.md P0-8, P0-11。
 */

import type { ApiResponse, PageResponse } from './types';

const BASE_URL = process.env.NEXT_PUBLIC_API_URL || '';

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
    res = await fetch(`${BASE_URL}${url}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
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
  const res = await fetch(`${BASE_URL}${url}`);
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
