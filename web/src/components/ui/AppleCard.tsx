/** AppleCard — 白底 + hairline 边框 + 18px 圆角，支持键盘可访问的点击交互。 */
import { type ReactNode, type HTMLAttributes, type KeyboardEvent } from 'react';

interface AppleCardProps extends HTMLAttributes<HTMLDivElement> {
  padding?: string;
  children: ReactNode;
}

export function AppleCard({
  padding = '24px',
  children,
  className = '',
  onClick,
  onKeyDown,
  style,
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
    'bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)]',
    isInteractive ? 'cursor-pointer hover:shadow-[0_2px_8px_rgba(0,0,0,0.06)] transition-shadow focus-visible:shadow-[0_0_0_3px_rgba(0,102,204,0.12)]' : '',
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
      style={{ padding, ...style }}
      {...rest}
    >
      {children}
    </div>
  );
}
