import { apiFetch } from './client';

export function getConfig(key: string) { return apiFetch<unknown>(`/api/v1/admin/configs/${key}`); }
export function setConfig(key: string, value: unknown) { return apiFetch<null>(`/api/v1/admin/configs/${key}`, { method: 'PUT', body: JSON.stringify({ value }) }); }
