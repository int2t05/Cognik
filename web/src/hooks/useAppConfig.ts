/**
 * useAppConfig — 从后端读取系统配置，取不到时回落默认值。
 *
 * 依赖 getAllConfigs（并行 GET /api/v1/admin/configs/:key），
 * 非管理员页面会 401，此时静默回落默认值。
 */
'use client';

import useSWR from 'swr';
import { SYSTEM_CONFIG_DEFAULTS, type SystemConfigKey } from '@/lib/config/defaults';
import { getAllConfigs } from '@/lib/api/config';

/** 合并后的配置值映射 */
type ConfigMap = Record<string, unknown>;

/**
 * 获取指定系统配置项的值。
 *
 * @param keys 需要获取的配置键列表
 * @returns 合并后的配置映射（后端值优先，取不到回落默认值）
 *
 * 使用示例：
 *   const { config, isLoading } = useAppConfig(['app_name']);
 *   const appName = (config.app_name as string) || 'OpsMind';
 */
export function useAppConfig(keys: SystemConfigKey[]) {
  const cacheKey = keys.length > 0 ? `app-config:${keys.join(',')}` : null;

  const { data, error, isLoading, mutate } = useSWR(
    cacheKey,
    () => getAllConfigs(keys as string[]),
    {
      revalidateOnFocus: false,
      dedupingInterval: 30_000,
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

/**
 * 获取单个配置项的值（便捷方法）。
 *
 * @returns 配置值（后端值优先，取不到回落默认值）
 */
export function useConfigValue<K extends SystemConfigKey>(key: K) {
  const { config, isLoading } = useAppConfig([key]);
  return {
    value: config[key] as (typeof SYSTEM_CONFIG_DEFAULTS)[K] | undefined,
    isLoading,
  };
}
