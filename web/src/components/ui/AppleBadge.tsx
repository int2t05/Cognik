/** AppleBadge — 语义状态标签。使用 CSS 变量适配亮/暗双主题。 */
type BadgeVariant = 'success' | 'warning' | 'error' | 'info' | 'neutral';

function badgeStyle(v: BadgeVariant): React.CSSProperties {
  return {
    backgroundColor: `var(--badge-${v}-bg)`,
    color: `var(--badge-${v}-text)`,
  };
}

export function AppleBadge({
  variant = 'neutral',
  label,
}: {
  variant?: BadgeVariant;
  label: string;
}) {
  return (
    <span
      className="inline-flex items-center gap-1 px-2.5 py-0.5 text-xs font-medium rounded-[var(--radius-pill)]"
      style={badgeStyle(variant)}
    >
      {label}
    </span>
  );
}
