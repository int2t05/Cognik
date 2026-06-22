import { apiFetch } from './client';

export function getConfig(key: string) { return apiFetch<unknown>(`/api/v1/admin/configs/${key}`); }
export function setConfig(key: string, value: unknown) { return apiFetch<null>(`/api/v1/admin/configs/${key}`, { method: 'PUT', body: JSON.stringify({ value }) }); }

/** 批量获取配置项，单 key 失败不影响其他，返回 { key, value } 数组。 */
export async function getAllConfigs(keys: string[]): Promise<{ key: string; value: unknown }[]> {
  const results = await Promise.allSettled(keys.map((key) => getConfig(key)));
  return results.map((r, i) => ({ key: keys[i], value: r.status === 'fulfilled' ? r.value : null }));
}
