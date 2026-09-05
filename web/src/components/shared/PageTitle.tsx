/** PageTitle — 内容页统一标题，内置 headline 字重 + 16px 下边距。
 *  走 cn/tailwind-merge，调用方传 mb-0 即可覆盖默认 mb-4（无需 !important）。
 *  与图标按钮同行（flex items-center）时务必传 className="mb-0" 避免垂直偏移。 */
import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export function PageTitle({ children, className }: { children: ReactNode; className?: string }) {
  return <h1 className={cn('text-headline font-semibold text-[var(--color-ink)] mb-4', className)}>{children}</h1>;
}
