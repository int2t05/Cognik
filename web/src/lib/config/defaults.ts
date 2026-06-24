/**
 * 系统配置前端默认值。
 *
 * 与后端 validConfigKeys 9 个键完全对齐，作为后端取不到值时的回落。
 * 后端新增配置键时，需同步更新此文件。
 */

export const SYSTEM_CONFIG_DEFAULTS = {
  app_name: 'OpsMind',
  'ai.rag_enabled': true,
  'ai.top_k': 5,
  'ai.threshold': 0.6,
  'ai.max_history_messages': 10,
  'ai.rag_query_rewrite': true,
  'ai.rag_multi_route': true,
  'ai.rag_hybrid': true,
  'ai.rag_rerank': true,
} as const;

export type SystemConfigKey = keyof typeof SYSTEM_CONFIG_DEFAULTS;

/** 获取单个配置项的默认值。 */
export function getDefaultConfig<K extends SystemConfigKey>(key: K): (typeof SYSTEM_CONFIG_DEFAULTS)[K] {
  return SYSTEM_CONFIG_DEFAULTS[key];
}

/** 获取应用名称（便捷方法）。 */
export function getAppName(): string {
  return SYSTEM_CONFIG_DEFAULTS.app_name;
}
