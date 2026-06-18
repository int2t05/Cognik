import { apiFetch } from './client';

export function getConfig(key: string) { return apiFetch<unknown>(`/api/v1/admin/configs/${key}`); }
export function setConfig(key: string, value: unknown) { return apiFetch<null>(`/api/v1/admin/configs/${key}`, { method: 'PUT', body: JSON.stringify({ value }) }); }

/** 批量获取多个配置项（并行请求），减少 SWR 调用次数。 */
export async function getAllConfigs(keys: string[]): Promise<{ key: string; value: unknown }[]> {
  const results = await Promise.all(keys.map((key) => getConfig(key)));
  return keys.map((key, i) => ({ key, value: results[i] }));
}
