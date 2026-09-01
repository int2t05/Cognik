// 错误处理 helper — 统一从 unknown 提取错误消息，避免每处 catch 重复 instanceof 判断。
/** 从 unknown 错误提取消息，失败时返回 fallback。 */
export function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}
