/**
 * 错误处理 helper —— 统一从 unknown 提取错误消息。
 * errorMessage：返回原始 message（后端透传 / JS 错误）。
 * translateError：客户端侧错误（ApiError 带 messageKey）按当前 locale 翻译（全路径键）；
 * 其余返回原始 message，非 Error 时返回 fallback。
 */
import { ApiError } from './client';

type Translator = (key: string, values?: Record<string, string | number>) => string;

/** 从 unknown 错误提取原始消息，失败时返回 fallback。 */
export function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

/**
 * 按当前 locale 翻译错误：ApiError 带 messageKey 时翻译（全路径键，如 error.authExpired），
 * 否则返回原始 message（后端消息透传 / JS 错误），非 Error 时返回 fallback。
 */
export function translateError(err: unknown, t: Translator, fallback = ''): string {
  if (err instanceof ApiError && err.messageKey) {
    return t(err.messageKey, err.messageParams);
  }
  return err instanceof Error ? err.message : fallback;
}
