/** AppleTable — 无边框 + 行悬浮 + loading skeleton + empty state */
'use client';

import { type ReactNode } from 'react';
import { AppleSpinner } from './AppleSpinner';

interface Column<T> {
  key: string;
  title: string;
  width?: string;
  render?: (row: T) => ReactNode;
}

interface AppleTableProps<T> {
  columns: Column<T>[];
  data: T[];
  loading?: boolean;
  rowKey: keyof T | ((row: T) => string | number);
  emptyText?: string;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function AppleTable<T extends Record<string, any>>({
  columns,
  data,
  loading = false,
  rowKey,
  emptyText = '暂无数据',
}: AppleTableProps<T>) {
  const getKey = (row: T): string | number => {
    if (typeof rowKey === 'function') return rowKey(row);
    return String(row[rowKey] ?? '');
  };

  return (
    <div className="bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)] overflow-x-auto">
      <table className="w-full border-collapse text-body">
        <thead>
          <tr>
            {columns.map((col) => (
              <th
                key={col.key}
                className="text-left text-caption font-semibold text-[var(--color-text-muted-48)] px-3 py-2.5 border-b border-[var(--color-hairline)] whitespace-nowrap"
                style={{ width: col.width }}
              >
                {col.title}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="[&>tr:last-child>td]:border-b-0">
          {loading ? (
            <tr>
              <td colSpan={columns.length} className="py-10 text-center">
                <AppleSpinner />
              </td>
            </tr>
          ) : data.length === 0 ? (
            <tr>
              <td colSpan={columns.length} className="py-12 text-center text-[var(--color-text-muted-48)] text-caption">
                {emptyText}
              </td>
            </tr>
          ) : (
            data.map((row) => (
              <tr key={getKey(row)} className="hover:bg-[var(--color-pearl)]">
                {columns.map((col) => (
                  <td key={col.key} className="px-3 py-2.5 border-b border-[var(--color-divider-soft)] text-[var(--color-ink)] text-caption">
                    {col.render ? col.render(row) : String(row[col.key] ?? '')}
                  </td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
