import { apiFetch, apiFetchPage } from './client';

export interface Role { id: number; name: string; description: string; permissions: string[]; created_at: string; updated_at: string; }
export interface Menu { id: number; name: string; path: string; icon: string; parent_id: number; sort_order: number; type: string; }

export function getRoleList(page: number) { return apiFetchPage<Role>(`/api/v1/admin/roles?page=${page}&page_size=10`); }
export function createRole(data: Record<string, unknown>) { return apiFetch<null>('/api/v1/admin/roles', { method: 'POST', body: JSON.stringify(data) }); }
export function updateRole(id: number, data: Record<string, unknown>) { return apiFetch<null>(`/api/v1/admin/roles/${id}`, { method: 'PUT', body: JSON.stringify(data) }); }
export function getRoleDetail(id: number) { return apiFetch<Role>(`/api/v1/admin/roles/${id}`); }
export function deleteRole(id: number) { return apiFetch<null>(`/api/v1/admin/roles/${id}`, { method: 'DELETE' }); }
export function updateRoleMenus(id: number, menuIds: number[]) { return apiFetch<null>(`/api/v1/admin/roles/${id}/menus`, { method: 'PUT', body: JSON.stringify({ menu_ids: menuIds }) }); }
export function getMenus() { return apiFetch<Menu[]>('/api/v1/admin/menus'); }
