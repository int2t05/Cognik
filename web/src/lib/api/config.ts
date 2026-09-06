import { apiFetch } from './client';

/** 公开配置（无需认证），仅 app_name）。 */
export function getPublicConfig(key: string) { return apiFetch<unknown>(`/api/v1/public/configs/${key}`); }

export interface EnvConfigEntry {
  key: string;
  value: string;
}

/** 获取 .env 派生的全部配置项（API key 脱敏）。 */
export function getEnvConfigs() {
  return apiFetch<EnvConfigEntry[]>('/api/v1/admin/configs/env');
}

/** 更新 .env 配置项并触发热重建。 */
export function updateEnvConfig(key: string, value: string) {
  return apiFetch<null>('/api/v1/admin/configs/env', { method: 'PUT', body: JSON.stringify({ key, value }) });
}
