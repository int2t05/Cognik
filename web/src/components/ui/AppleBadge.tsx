/** AppleBadge — 语义状态标签 */
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
