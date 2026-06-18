/** 通用格式化工具 */

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
