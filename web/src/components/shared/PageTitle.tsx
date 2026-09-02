/** PageTitle — 全站统一页面标题，内置 headline 字重 + 16px 下边距。 */
import type { ReactNode } from 'react';

export function PageTitle({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <h1 className={`text-headline font-semibold text-[var(--color-ink)] mb-4 ${className}`}>{children}</h1>;
}
