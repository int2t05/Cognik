/**
 * 系统配置前端默认值，作为后端取不到值时的回落。
 */

export const SYSTEM_CONFIG_DEFAULTS = {
  app_name: 'Cognik',
  'ai.rag_enabled': true,
  'ai.top_k': 5,
  'ai.confidence_threshold_low': 0.40,
  'ai.confidence_threshold_high': 0.70,
  'ai.max_history_messages': 10,
  'ai.rag_query_rewrite': true,
  'ai.rag_multi_route': true,
  'ai.rag_hybrid': true,
  'ai.rag_rerank': true,
  'ai.enable_thinking': false,
} as const;

export type SystemConfigKey = keyof typeof SYSTEM_CONFIG_DEFAULTS;

/** 获取应用名称（便捷方法）。 */
export function getAppName(): string {
  return SYSTEM_CONFIG_DEFAULTS.app_name;
}
