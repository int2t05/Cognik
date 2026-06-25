/**
 * CRUD 工厂 — 为 RESTful 资源生成带类型的 CRUD 方法，消除 11 个 API 模块中的重复。
 *
 * 每项资源仅定义特殊操作（如 batchDelete、freeze、upload 等），
 * 标准 CRUD 由工厂自动生成，确保 URL 拼接、分页参数、错误处理一致。
 *
 * 为什么用工厂而非 class：
 * 工厂返回纯函数对象，消费者按需解构，零 this 绑定开销，tree-shaking 友好。
 */

import { apiFetch, apiFetchPage } from './client';
import { PAGE_SIZE } from './constants';

export interface CrudConfig {
  /** API 基础路径，如 '/api/v1/admin/users' */
  basePath: string;
}

export interface PaginatedResult<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

export interface CrudApi<T extends { id: number | string }> {
  getList: (page: number) => Promise<PaginatedResult<T>>;
  getDetail: (id: number | string) => Promise<T>;
  create: <R = null>(data: Record<string, unknown>) => Promise<R>;
  update: <R = null>(id: number | string, data: Record<string, unknown>) => Promise<R>;
  delete: <R = null>(id: number | string) => Promise<R>;
}

/**
 * 为指定资源路径创建标准 CRUD 方法。
 *
 * 特殊操作（批量删除、冻结、上传文件等）应在资源模块中单独定义，
 * 导入本工厂生成的 getList/getDetail/create/update/delete 并按需扩展。
 */
export function createCrudApi<T extends { id: number | string }>(
  config: CrudConfig,
): CrudApi<T> {
  const { basePath } = config;

  return {
    getList: (page: number) =>
      apiFetchPage<T>(`${basePath}?page=${page}&page_size=${PAGE_SIZE}`),

    getDetail: (id: number | string) =>
      apiFetch<T>(`${basePath}/${id}`),

    create: <R = null>(data: Record<string, unknown>) =>
      apiFetch<R>(basePath, { method: 'POST', body: JSON.stringify(data) }),

    update: <R = null>(id: number | string, data: Record<string, unknown>) =>
      apiFetch<R>(`${basePath}/${id}`, { method: 'PUT', body: JSON.stringify(data) }),

    delete: <R = null>(id: number | string) =>
      apiFetch<R>(`${basePath}/${id}`, { method: 'DELETE' }),
  };
}

/**
 * 为带关键词筛选的列表 API 构建查询 URL。
 * 提取自 user/ticket/knowledge/audit 中逐字重复的模式。
 */
export function buildFilterUrl(
  basePath: string,
  page: number,
  params?: Record<string, string | number | undefined>,
): string {
  const url = `${basePath}?page=${page}&page_size=${PAGE_SIZE}`;
  const extra = Object.entries(params ?? {})
    .filter(([, v]) => v !== undefined && v !== '' && v !== -1)
    .map(([k, v]) => `${k}=${encodeURIComponent(String(v))}`)
    .join('&');
  return extra ? `${url}&${extra}` : url;
}
