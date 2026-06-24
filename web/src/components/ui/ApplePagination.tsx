/** ApplePagination — 图标驱动分页，紧凑精致。 */
'use client';

import { ChevronLeft, ChevronRight } from 'lucide-react';

interface ApplePaginationProps {
  page: number;
  pageSize: number;
  total: number;
  pageSizeOptions?: number[];
  onChange: (page: number, pageSize: number) => void;
}

export function ApplePagination({
  page,
  pageSize,
  total,
  pageSizeOptions = [10, 20, 50],
  onChange,
}: ApplePaginationProps) {
  const totalPages = Math.ceil(total / pageSize);
  if (total === 0) return null;

  const pages = getVisiblePages(page, totalPages);

  return (
    <div className="flex items-center justify-between py-4 text-caption text-[var(--color-text-muted-48)]">
      <span>共 {total} 条</span>

      <div className="flex items-center bg-foreground rounded-md p-2 gap-2">
        <PageBtn disabled={page <= 1} onClick={() => onChange(page - 1, pageSize)} aria-label="上一页">
          <ChevronLeft size={16} />
        </PageBtn>

        {pages.map((p, i) =>
          p === 0 ? (
            <span key={`gap-${i}`} className="w-7 text-center text-[var(--color-text-muted-48)]">…</span>
          ) : (
            <PageBtn key={p} active={p === page} onClick={() => onChange(p, pageSize)}>
              {p}
            </PageBtn>
          )
        )}

        <PageBtn disabled={page >= totalPages} onClick={() => onChange(page + 1, pageSize)} aria-label="下一页">
          <ChevronRight size={16} />
        </PageBtn>
      </div>

      <select
        aria-label="每页条数"
        className="px-2 py-1 text-caption rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none cursor-pointer transition focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]"
        value={pageSize}
        onChange={(e) => onChange(1, Number(e.target.value))}
      >
        {pageSizeOptions.map((s) => (
          <option key={s} value={s}>{s} 条/页</option>
        ))}
      </select>
    </div>
  );
}

/** 计算可见页码：首尾 + 当前 ±1，其余用 0 表示省略号 */
function getVisiblePages(page: number, total: number): number[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);

  const result: number[] = [1];
  const start = Math.max(2, page - 1);
  const end = Math.min(total - 1, page + 1);

  if (start > 2) result.push(0);
  for (let i = start; i <= end; i++) result.push(i);
  if (end < total - 1) result.push(0);
  result.push(total);

  return result;
}

function PageBtn({
  active, disabled, onClick, children, ...rest
}: {
  active?: boolean; disabled?: boolean; onClick: () => void; children: React.ReactNode;
} & React.ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      {...rest}
      className={`px-4 h-8 flex items-center justify-center text-caption rounded-[var(--radius-pill)] border-0 font-sans cursor-pointer transition active:scale-95 ${
        active
          ? 'bg-[var(--color-accent)] text-[var(--color-on-accent)]'
          : 'text-[var(--color-ink)] hover:bg-[var(--color-divider-soft)]'
      } ${disabled ? 'opacity-40 disabled:cursor-not-allowed' : ''}`}
    >
      {children}
    </button>
  );
}
