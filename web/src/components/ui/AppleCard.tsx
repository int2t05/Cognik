/** AppleCard — 白底 + hairline 边框 + 18px 圆角 + 24px 内边距，支持键盘可访问的点击交互。 */
import { type ReactNode, type HTMLAttributes, type KeyboardEvent } from 'react';

interface AppleCardProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
}

export function AppleCard({
  children,
  className = '',
  onClick,
  onKeyDown,
  ...rest
}: AppleCardProps) {
  const isInteractive = !!onClick;

  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (onKeyDown) onKeyDown(e);
    if (isInteractive && (e.key === 'Enter' || e.key === ' ')) {
      e.preventDefault();
      onClick!(e as unknown as React.MouseEvent<HTMLDivElement>);
    }
  };

  const classNames = [
    'bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)] p-6',
    isInteractive ? 'cursor-pointer hover:shadow-[var(--shadow-card-hover)] hover:-translate-y-px transition-all focus-visible:shadow-[var(--focus-ring)]' : '',
    className,
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <div
      className={classNames}
      onClick={onClick}
      onKeyDown={isInteractive ? handleKeyDown : onKeyDown}
      tabIndex={isInteractive ? 0 : undefined}
      role={isInteractive ? 'button' : undefined}
      {...rest}
    >
      {children}
    </div>
  );
}
