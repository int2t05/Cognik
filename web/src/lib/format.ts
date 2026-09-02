/** 安全格式化百分比，处理 null/undefined */
export function formatPercent(value: number | null | undefined): string {
  if (value == null || isNaN(value)) return '—';
  return `${(value * 100).toFixed(0)}%`;
}
