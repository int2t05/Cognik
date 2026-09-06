/** useAppConfig 从后端读取 .env 配置，取不到回落默认值。 */
'use client';

import useSWR from 'swr';
import { SYSTEM_CONFIG_DEFAULTS, type SystemConfigKey } from '@/lib/config/defaults';
import { getEnvConfigs } from '@/lib/api/config';

type ConfigMap = Record<string, unknown>;

/** 获取指定配置项，后端值优先，取不到回落默认值。 */
export function useAppConfig(keys: SystemConfigKey[]) {
  const cacheKey = keys.length > 0 ? `app-config:${keys.join(',')}` : null;

  const { data, error, isLoading, mutate } = useSWR(
    cacheKey,
    () => getEnvConfigs(),
    {
      revalidateOnFocus: false,
      refreshInterval: 900_000,
      dedupingInterval: 300_000,
      errorRetryCount: 1,
    },
  );

  const config: ConfigMap = {};
  for (const key of keys) {
    const backendValue = data?.find((c) => c.key === key)?.value;
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
