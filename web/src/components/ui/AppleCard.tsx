/** AppleCard — 白底 + hairline 边框 + 18px 圆角 */
import { type ReactNode, type HTMLAttributes } from 'react';

interface AppleCardProps extends HTMLAttributes<HTMLDivElement> {
  padding?: string;
  children: ReactNode;
}

export function AppleCard({
  padding = 'var(--space-lg)',
  children,
  className = '',
  onClick,
  style,
  ...rest
}: AppleCardProps) {
  const classNames = [
    'bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)]',
    onClick ? 'cursor-pointer hover:shadow-[0_2px_8px_rgba(0,0,0,0.06)] transition-shadow' : '',
    className,
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <div className={classNames} onClick={onClick} style={{ padding, ...style }} {...rest}>
      {children}
    </div>
  );
}
