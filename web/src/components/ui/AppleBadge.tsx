/** AppleBadge — 语义状态标签。
 *
 * TODO(ui/AppleBadge): variantColors 使用硬编码 hex 色值，暗色模式下背景/文字不会自适应。
 * 应改用 CSS 变量（如 var(--color-success-bg)）或 prefers-color-scheme 媒体查询。 */
type BadgeVariant = 'success' | 'warning' | 'error' | 'info' | 'neutral';

const variantColors: Record<BadgeVariant, { bg: string; text: string }> = {
  success: { bg: '#e8f5e9', text: '#2e7d32' },
  warning: { bg: '#fff3e0', text: '#e65100' },
  error: { bg: '#fce4ec', text: '#c62828' },
  info: { bg: '#e3f2fd', text: '#1565c0' },
  neutral: { bg: '#f5f5f5', text: '#616161' },
};

export function AppleBadge({
  variant = 'neutral',
  label,
}: {
  variant?: BadgeVariant;
  label: string;
}) {
  const c = variantColors[variant];
  return (
    <span
      style={{
        display: 'inline-block',
        padding: '2px 10px',
        fontSize: 12,
        fontWeight: 500,
        lineHeight: '20px',
        borderRadius: 'var(--radius-pill)',
        background: c.bg,
        color: c.text,
      }}
    >
      {label}
    </span>
  );
}
