/**
 * 系统配置前端默认值，作为 .env 取不到值时的回落。
 */
export const SYSTEM_CONFIG_DEFAULTS = {
  app_name: 'Cognik',
  'ai.rag_enabled': true,
  'ai.top_k': 5,
  'ai.confidence_threshold': 0.40,
  'ai.confidence_threshold_high': 0.70,
  'ai.max_history_messages': 10,
} as const;

export type SystemConfigKey = keyof typeof SYSTEM_CONFIG_DEFAULTS;

/** 获取应用名称（便捷方法）。 */
export function getAppName(): string {
  return SYSTEM_CONFIG_DEFAULTS.app_name;
}
