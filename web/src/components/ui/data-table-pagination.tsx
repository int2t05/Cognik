'use client';
// DataTablePagination — 分页器 + 页大小选择，基于 shadcn Pagination + Select（Radix，键盘可达）。
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { Pagination, PaginationContent, PaginationItem, PaginationLink, PaginationEllipsis } from '@/components/ui/pagination';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

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

  return (
    <div className="flex items-center justify-between flex-wrap gap-3 py-3">
      <span className="text-fine text-[var(--color-text-muted-48)]">第 {start}-{end} / {total} 条</span>
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2">
          <span className="text-fine text-[var(--color-text-muted-48)]">每页</span>
          <Select value={String(pageSize)} onValueChange={(v) => onChange(1, Number(v))}>
            <SelectTrigger className="h-8 w-[70px]"><SelectValue /></SelectTrigger>
            <SelectContent>
              {pageSizeOptions.map((n) => (
                <SelectItem key={n} value={String(n)}>{n}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <Pagination className="mx-0 w-auto justify-end">
          <PaginationContent>
            <PaginationItem>
              <PaginationLink
                size="icon"
                href="#"
                aria-label="上一页"
                aria-disabled={page <= 1}
                className={page <= 1 ? 'pointer-events-none opacity-40' : ''}
                onClick={(e) => { e.preventDefault(); if (page > 1) onChange(page - 1, pageSize); }}
              >
                <ChevronLeft className="size-4" />
              </PaginationLink>
            </PaginationItem>
            {visible.map((p, i) =>
              p === 0 ? (
                <PaginationItem key={`e${i}`}><PaginationEllipsis /></PaginationItem>
              ) : (
                <PaginationItem key={p}>
                  <PaginationLink
                    size="icon"
                    href="#"
                    isActive={p === page}
                    aria-label={`第 ${p} 页`}
                    onClick={(e) => { e.preventDefault(); onChange(p, pageSize); }}
                  >
                    {p}
                  </PaginationLink>
                </PaginationItem>
              )
            )}
            <PaginationItem>
              <PaginationLink
                size="icon"
                href="#"
                aria-label="下一页"
                aria-disabled={page >= totalPages}
                className={page >= totalPages ? 'pointer-events-none opacity-40' : ''}
                onClick={(e) => { e.preventDefault(); if (page < totalPages) onChange(page + 1, pageSize); }}
              >
                <ChevronRight className="size-4" />
              </PaginationLink>
            </PaginationItem>
          </PaginationContent>
        </Pagination>
      </div>
    </div>
  );
}
