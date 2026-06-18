import { apiFetch, apiFetchPage } from './client';

export interface User { id: number; username: string; real_name: string; phone: string; email: string; status: number; first_login: boolean; roles: string[]; created_at: string; updated_at: string; }

export function getUserList(page: number, keyword?: string) {
  let url = `/api/v1/admin/users?page=${page}&page_size=10`;
  if (keyword) url += `&keyword=${encodeURIComponent(keyword)}`;
  return apiFetchPage<User>(url);
}
export function createUser(data: Record<string, unknown>) { return apiFetch<null>('/api/v1/admin/users', { method: 'POST', body: JSON.stringify(data) }); }
export function updateUser(id: number, data: Record<string, unknown>) { return apiFetch<null>(`/api/v1/admin/users/${id}`, { method: 'PUT', body: JSON.stringify(data) }); }
export function freezeUser(id: number) { return apiFetch<null>(`/api/v1/admin/users/${id}/freeze`, { method: 'PATCH' }); }
export function getUserDetail(id: number) { return apiFetch<User>(`/api/v1/admin/users/${id}`); }
export function unfreezeUser(id: number) { return apiFetch<null>(`/api/v1/admin/users/${id}/unfreeze`, { method: 'PATCH' }); }
