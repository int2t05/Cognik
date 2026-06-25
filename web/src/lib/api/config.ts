import { apiFetch } from './client';

/** 公开配置（无需认证），仅限白名单内的 key（如 app_name）。 */
export function getPublicConfig(key: string) { return apiFetch<unknown>(`/api/v1/public/configs/${key}`); }

export function getConfig(key: string) { return apiFetch<unknown>(`/api/v1/admin/configs/${key}`); }
export function setConfig(key: string, value: unknown) { return apiFetch<null>(`/api/v1/admin/configs/${key}`, { method: 'PUT', body: JSON.stringify({ value }) }); }

/** 批量获取配置项，单 key 失败不影响其他，返回 { key, value } 数组。 */
export async function getAllConfigs(keys: string[]): Promise<{ key: string; value: unknown }[]> {
  const results = await Promise.allSettled(keys.map((key) => getConfig(key)));
  return results.map((r, i) => ({ key: keys[i], value: r.status === 'fulfilled' ? r.value : null }));
}

export interface ComputeThresholdsResult {
  p30: number; p70: number; sample_count: number;
  date_from?: string; date_to?: string; warning?: string;
}

/** 从历史数据计算置信度分位数阈值。 */
export function computeThresholds(days: number) {
  return apiFetch<ComputeThresholdsResult>('/api/v1/admin/confidence/compute-thresholds', {
    method: 'POST', body: JSON.stringify({ days }),
  });
}
