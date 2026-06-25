/**
 * User API — 用户 CRUD + 批量删除 + 冻结/恢复。
 * 标准 CRUD 由 createCrudApi 工厂生成，本模块仅保留特殊操作。
 */
import { apiFetch, apiFetchPage } from './client';
import { PAGE_SIZE } from './constants';

export interface User { id: number; username: string; real_name: string; phone: string; email: string; status: number; first_login: boolean; roles: string[]; created_at: string; updated_at: string; }

export function getUserList(page: number, keyword?: string) {
  let url = `/api/v1/admin/users?page=${page}&page_size=${PAGE_SIZE}`;
  if (keyword) url += `&keyword=${encodeURIComponent(keyword)}`;
  return apiFetchPage<User>(url);
}
export function getUserDetail(id: number) { return apiFetch<User>(`/api/v1/admin/users/${id}`); }
export function createUser(data: Record<string, unknown>) { return apiFetch<null>('/api/v1/admin/users', { method: 'POST', body: JSON.stringify(data) }); }
export function updateUser(id: number, data: Record<string, unknown>) { return apiFetch<null>(`/api/v1/admin/users/${id}`, { method: 'PUT', body: JSON.stringify(data) }); }
export function freezeUser(id: number) { return apiFetch<null>(`/api/v1/admin/users/${id}/freeze`, { method: 'PATCH' }); }
export function unfreezeUser(id: number) { return apiFetch<null>(`/api/v1/admin/users/${id}/unfreeze`, { method: 'PATCH' }); }
export function batchDeleteUsers(ids: number[]) {
  return apiFetch<null>('/api/v1/admin/users/batch-delete', { method: 'POST', body: JSON.stringify({ ids }) });
}
