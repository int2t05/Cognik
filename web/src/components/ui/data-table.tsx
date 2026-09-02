'use client';
// DataTable — 通用数据表格。基于 shadcn Table 原语 + TanStack Table v9，内置 skeleton/empty 态。
import { useTable, tableFeatures, type ColumnDef, type TableFeatures, type RowData } from '@tanstack/react-table';
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '@/components/ui/table';
import { Skeleton } from '@/components/ui/skeleton';

interface DataTableProps<TData extends RowData> {
  columns: ColumnDef<TableFeatures, TData>[];
  data: TData[];
  loading?: boolean;
  emptyText?: string;
  /** 加载骨架行数 */
  skeletonRows?: number;
}

export function DataTable<TData extends RowData>({
  columns,
  data,
  loading,
  emptyText = '暂无数据',
  skeletonRows = 5,
}: DataTableProps<TData>) {
  const table = useTable({
    data,
    columns,
    features: tableFeatures({}),
  });

  return (
    <div className="rounded-[var(--radius-lg)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] overflow-hidden">
      <Table>
        <TableHeader>
          {table.getHeaderGroups().map((hg) => (
            <TableRow key={hg.id} className="border-b border-[var(--color-hairline)]">
              {hg.headers.map((h) => (
                <TableHead key={h.id} className="text-fine uppercase tracking-wide text-[var(--color-text-muted-48)] font-medium h-9 px-3">
                  {h.isPlaceholder ? null : <table.FlexRender header={h} />}
                </TableHead>
              ))}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {loading ? (
            Array.from({ length: skeletonRows }).map((_, i) => (
              <TableRow key={i} className="border-b border-[var(--color-divider-soft)]">
                {columns.map((_, j) => (
                  <TableCell key={j} className="px-3 py-2.5"><Skeleton className="h-4 w-full" /></TableCell>
                ))}
              </TableRow>
            ))
          ) : table.getRowModel().rows.length === 0 ? (
            <TableRow>
              <TableCell colSpan={columns.length} className="h-20 text-center text-[var(--color-text-muted-48)] text-caption">
                {emptyText}
              </TableCell>
            </TableRow>
          ) : (
            table.getRowModel().rows.map((row) => (
              <TableRow key={row.id} className="border-b border-[var(--color-divider-soft)] hover:bg-[var(--color-pearl)]">
                {row.getAllCells().map((cell) => (
                  <TableCell key={cell.id} className="px-3 py-2.5 text-callout text-[var(--color-ink)]">
                    <table.FlexRender cell={cell} />
                  </TableCell>
                ))}
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </div>
  );
}
