/** PageTitle — 全站统一页面标题，内置 hero 字重 + 20px 下边距。 */
import type { ReactNode } from 'react';

export function PageTitle({ children, className = '' }: { children: ReactNode; className?: string }) {
  return <h1 className={`text-display font-semibold text-[var(--color-ink)] mb-5 ${className}`}>{children}</h1>;
}
