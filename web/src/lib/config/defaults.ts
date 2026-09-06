/**
 * 系统配置前端默认值，作为 .env 取不到值时的回落。
 * Key 与后端 GetEnvConfigs 返回的真实 .env 变量名一致，避免映射漂移。
 */
export const SYSTEM_CONFIG_DEFAULTS = {
  COGNIK_APP_NAME: 'Cognik',
  COGNIK_AI_RAG_ENABLED: true,
  COGNIK_AI_TOP_K: 5,
  COGNIK_AI_CONFIDENCE_THRESHOLD: 0.40,
  COGNIK_AI_MAX_HISTORY_MESSAGES: 10,
} as const;

export type SystemConfigKey = keyof typeof SYSTEM_CONFIG_DEFAULTS;

/** 获取应用名称（便捷方法）。 */
export function getAppName(): string {
  return SYSTEM_CONFIG_DEFAULTS.COGNIK_APP_NAME;
}
