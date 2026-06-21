/** 通用格式化工具与常量 */

/** 紧急程度标签 — 索引对应后端 urgency 字段值（1=低, 2=中, 3=高） */
export const URGENCY_LABELS = ['', '低', '中', '高'] as const;

/** 安全格式化百分比，处理 null/undefined */
export function formatPercent(value: number | null | undefined): string {
  if (value == null || isNaN(value)) return '—';
  return `${(value * 100).toFixed(0)}%`;
}

/** 截断文本 */
export function truncate(text: string, maxLen: number): string {
  if (text.length <= maxLen) return text;
  return text.slice(0, maxLen) + '…';
}
