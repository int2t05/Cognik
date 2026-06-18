/** AppleBadge — 语义状态标签。CSS Module + 语义色 token */
import styles from './AppleBadge.module.css';

type BadgeVariant = 'success' | 'warning' | 'error' | 'info' | 'neutral';

export function AppleBadge({
  variant = 'neutral',
  label,
}: {
  variant?: BadgeVariant;
  label: string;
}) {
  return (
    <span className={`${styles.badge} ${styles[variant]}`}>
      {label}
    </span>
  );
}
