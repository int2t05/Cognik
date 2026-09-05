/**
 * 格式化 YYYY-MM-DD HH:MM:SS 或 ISO 字符串为可读格式。
 * locale 默认 zh-CN；调用方（client 组件）传入 useLocale() 结果以按当前语言格式化。
 */
export function formatDate(dateStr: string | null | undefined, locale: string = 'zh-CN'): string {
  if (!dateStr) return '—';
  try {
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return dateStr;
    return d.toLocaleDateString(locale, {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return dateStr;
  }
}
