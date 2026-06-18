/** AppleCard — 白底 + hairline 边框 + 18px 圆角 */
import { type ReactNode, type HTMLAttributes } from 'react';
import styles from './AppleCard.module.css';

interface AppleCardProps extends HTMLAttributes<HTMLDivElement> {
  padding?: string;
  children: ReactNode;
}

export function AppleCard({ padding = 'var(--space-lg)', children, className = '', onClick, style, ...rest }: AppleCardProps) {
  const classNames = [styles.card, onClick ? styles.clickable : '', className].filter(Boolean).join(' ');
  return (
    <div
      className={classNames}
      onClick={onClick}
      style={{ '--card-padding': padding, ...style } as React.CSSProperties}
      {...rest}
    >
      {children}
    </div>
  );
}
