'use client';
// DataTablePagination — GitHub 风格分页：左侧计数+页大小 + 居中页码导航。
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

interface DataTablePaginationProps {
  page: number;
  pageSize: number;
  total: number;
  pageSizeOptions?: number[];
  onChange: (page: number, pageSize: number) => void;
}

/** 计算可见页码（≤7 全显示，否则首尾 + 当前±1 + 省略号；0 表示省略号占位） */
function getVisiblePages(page: number, totalPages: number): number[] {
  if (totalPages <= 7) return Array.from({ length: totalPages }, (_, i) => i + 1);
  const pages: number[] = [1];
  const start = Math.max(2, page - 1);
  const end = Math.min(totalPages - 1, page + 1);
  if (start > 2) pages.push(0);
  for (let i = start; i <= end; i++) pages.push(i);
  if (end < totalPages - 1) pages.push(0);
  pages.push(totalPages);
  return pages;
}

export function DataTablePagination({
  page,
  pageSize,
  total,
  pageSizeOptions = [10, 20, 50],
  onChange,
}: DataTablePaginationProps) {
  if (total === 0) return null;
  const totalPages = Math.ceil(total / pageSize);
  const visible = getVisiblePages(page, totalPages);
  const start = (page - 1) * pageSize + 1;
  const end = Math.min(page * pageSize, total);

  // 页码按钮基础样式 — GitHub 风格：无边框、方形、hover 静音背景
  const btnBase = 'inline-flex items-center justify-center min-w-[28px] h-7 px-2 text-caption rounded-[var(--radius-sm)] transition-colors';
  const btnIdle = 'text-[var(--color-text-muted-48)] hover:bg-[var(--color-tile-1)] hover:text-[var(--color-ink)]';
  const btnActive = 'bg-[var(--color-tile-1)] text-[var(--color-ink)] font-medium';
  const btnDisabled = 'pointer-events-none opacity-30';

  return (
    <div className="flex items-center py-2 px-4 border-t border-[var(--color-divider-soft)]">
      {/* 左：计数 + 页大小 */}
      <div className="flex items-center gap-3 flex-1">
        <span className="text-fine text-[var(--color-text-muted-48)]">
          第 {start}-{end} 条，共 {total} 条
        </span>
        <Select value={String(pageSize)} onValueChange={(v) => onChange(1, Number(v))}>
          <SelectTrigger className="h-8 w-[68px] text-caption"><SelectValue /></SelectTrigger>
          <SelectContent>
            {pageSizeOptions.map((n) => (
              <SelectItem key={n} value={String(n)}>{n}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* 中：页码导航（居中，右侧 flex-1 平衡） */}
      <div className="flex items-center gap-0.5 justify-center">
        <button
          type="button"
          aria-label="上一页"
          disabled={page <= 1}
          onClick={() => page > 1 && onChange(page - 1, pageSize)}
          className={cn(btnBase, 'w-7', page <= 1 ? btnDisabled : btnIdle)}
        >
          <ChevronLeft className="size-4" />
        </button>
        {visible.map((p, i) =>
          p === 0 ? (
            <span key={`e${i}`} className={cn(btnBase, 'pointer-events-none')}>…</span>
          ) : (
            <button
              key={p}
              type="button"
              onClick={() => onChange(p, pageSize)}
              aria-current={p === page ? 'page' : undefined}
              className={cn(btnBase, p === page ? btnActive : btnIdle)}
            >
              {p}
            </button>
          )
        )}
        <button
          type="button"
          aria-label="下一页"
          disabled={page >= totalPages}
          onClick={() => page < totalPages && onChange(page + 1, pageSize)}
          className={cn(btnBase, 'w-7', page >= totalPages ? btnDisabled : btnIdle)}
        >
          <ChevronRight className="size-4" />
        </button>
      </div>

      {/* 右：平衡占位（与左侧 flex-1 对称，使中间居中） */}
      <div className="flex-1" />
    </div>
  );
}
