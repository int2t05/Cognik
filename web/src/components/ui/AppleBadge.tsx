/** AppleBadge — 语义状态标签。颜色+图标双重编码，兼容色觉障碍。 */
import type { CSSProperties } from 'react';
import { CheckCircle, AlertTriangle, XCircle, Info, Minus } from 'lucide-react';

type BadgeVariant = 'success' | 'warning' | 'error' | 'info' | 'neutral';

const BADGE_ICONS: Record<BadgeVariant, React.ReactNode> = {
  success: <CheckCircle size={12} />,
  warning: <AlertTriangle size={12} />,
  error: <XCircle size={12} />,
  info: <Info size={12} />,
  neutral: <Minus size={12} />,
};

function badgeStyle(v: BadgeVariant): CSSProperties {
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
      className={`inline-flex items-center gap-1 px-2.5 py-0.5 text-fine font-semibold rounded-[var(--radius-pill)] ${className}`}
      style={badgeStyle(variant)}
    >
      {BADGE_ICONS[variant]}
      {label}
    </span>
  );
}
