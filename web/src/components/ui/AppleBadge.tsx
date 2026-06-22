/** AppleBadge — 语义状态标签。使用 CSS 变量适配亮/暗双主题。 */
// TODO: 显式 import type { CSSProperties } from 'react' 而非依赖全局 React 命名空间
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
  className = '',
}: {
  variant?: BadgeVariant;
  label: string;
  className?: string;
}) {
  return (
    <span
      className={`inline-flex items-center gap-1 px-2.5 py-0.5 text-fine font-medium rounded-[var(--radius-pill)] ${className}`}
      style={badgeStyle(variant)}
    >
      {label}
    </span>
  );
}
