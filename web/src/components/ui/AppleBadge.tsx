/** AppleBadge — 语义状态标签 */
type BadgeVariant = 'success' | 'warning' | 'error' | 'info' | 'neutral';

const variantClasses: Record<BadgeVariant, string> = {
  success: 'bg-green-100 text-green-700',
  warning: 'bg-orange-100 text-orange-700',
  error: 'bg-red-100 text-red-700',
  info: 'bg-blue-100 text-blue-700',
  neutral: 'bg-gray-100 text-gray-600',
};

export function AppleBadge({
  variant = 'neutral',
  label,
}: {
  variant?: BadgeVariant;
  label: string;
}) {
  return (
    <span
      className={`inline-flex items-center gap-1 px-2.5 py-0.5 text-xs font-medium rounded-[var(--radius-pill)] ${variantClasses[variant]}`}
    >
      {label}
    </span>
  );
}
