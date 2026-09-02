/** useAppConfig 从后端读取系统配置，非管理员 401 时静默回落默认值。 */
'use client';

import useSWR from 'swr';
import { SYSTEM_CONFIG_DEFAULTS, type SystemConfigKey } from '@/lib/config/defaults';
import { getAllConfigs } from '@/lib/api/config';

/** 合并后的配置值映射 */
type ConfigMap = Record<string, unknown>;

/** 获取指定系统配置项，后端值优先，取不到回落默认值。 */
export function useAppConfig(keys: SystemConfigKey[]) {
  const cacheKey = keys.length > 0 ? `app-config:${keys.join(',')}` : null;

  const { data, error, isLoading, mutate } = useSWR(
    cacheKey,
    () => getAllConfigs(keys as string[]),
    {
      revalidateOnFocus: false,
      refreshInterval: 900_000, // 15 分钟轮询，减少 DB 查询压力
      dedupingInterval: 300_000, // 5 分钟去重窗口
      errorRetryCount: 1, // 非管理页 401 只重试一次，避免无意义请求
    },
  );

  // 合并：后端值优先，取不到回落默认值
  const config: ConfigMap = {};
  for (const key of keys) {
    const backendValue = data?.find((c) => c.key === key && c.value !== null)?.value;
    config[key] = backendValue ?? SYSTEM_CONFIG_DEFAULTS[key];
  }

  return { config, error, isLoading, mutate };
}

/** 获取单个配置项，后端值优先，取不到回落默认值。 */
export function useConfigValue<K extends SystemConfigKey>(key: K) {
  const { config, isLoading } = useAppConfig([key]);
  return {
    value: config[key] as (typeof SYSTEM_CONFIG_DEFAULTS)[K] | undefined,
    isLoading,
  };
}
