/** AppleBadge — 语义状态标签。暗色模式通过 CSS 变量自适应。 */
type BadgeVariant = 'success' | 'warning' | 'error' | 'info' | 'neutral';

const variantColors: Record<BadgeVariant, { bg: string; text: string }> = {
  success: { bg: 'var(--badge-success-bg, #e8f5e9)', text: 'var(--badge-success-text, #2e7d32)' },
  warning: { bg: 'var(--badge-warning-bg, #fff3e0)', text: 'var(--badge-warning-text, #e65100)' },
  error:   { bg: 'var(--badge-error-bg, #fce4ec)',   text: 'var(--badge-error-text, #c62828)' },
  info:    { bg: 'var(--badge-info-bg, #e3f2fd)',    text: 'var(--badge-info-text, #1565c0)' },
  neutral: { bg: 'var(--badge-neutral-bg, #f5f5f5)', text: 'var(--badge-neutral-text, #616161)' },
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
