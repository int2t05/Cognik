/** AppleCard — 白底 + hairline 边框 + 18px 圆角 */
import { type ReactNode, type HTMLAttributes } from 'react';

interface AppleCardProps extends HTMLAttributes<HTMLDivElement> {
  padding?: string;
  children: ReactNode;
}

export function AppleCard({ padding = 'var(--space-lg)', children, style, ...rest }: AppleCardProps) {
  return (
    <div
      style={{
        background: 'var(--bg-canvas)',
        borderRadius: 'var(--radius-lg)',
        border: '1px solid var(--hairline)',
        padding,
        ...style,
      }}
      {...rest}
    >
      {children}
    </div>
  );
}
