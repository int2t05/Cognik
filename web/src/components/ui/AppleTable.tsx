/** AppleTable — 无边框 + 斑马行 + loading skeleton + empty state */
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
    <div style={{ overflowX: 'auto' }}>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr>
            {columns.map((col) => (
              <th
                key={col.key}
                style={{
                  padding: '10px 16px',
                  textAlign: 'left',
                  fontSize: 12,
                  fontWeight: 600,
                  letterSpacing: '-0.12px',
                  color: 'var(--text-muted-48)',
                  borderBottom: '1px solid var(--divider-soft)',
                  width: col.width,
                  whiteSpace: 'nowrap',
                }}
              >
                {col.title}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {loading ? (
            <tr>
              <td colSpan={columns.length} style={{ padding: 40, textAlign: 'center' }}>
                <div style={{ display: 'flex', justifyContent: 'center' }}><AppleSpinner /></div>
              </td>
            </tr>
          ) : data.length === 0 ? (
            <tr>
              <td colSpan={columns.length} style={{ padding: 40, textAlign: 'center', color: 'var(--text-muted-48)', fontSize: 14 }}>
                {emptyText}
              </td>
            </tr>
          ) : (
            data.map((row, i) => (
              <tr key={getKey(row)} style={{ background: i % 2 === 0 ? 'transparent' : 'var(--divider-soft)' }}>
                {columns.map((col) => (
                  <td
                    key={col.key}
                    style={{
                      padding: '10px 16px',
                      fontSize: 14,
                      lineHeight: 1.43,
                      color: 'var(--text-ink)',
                      borderBottom: '1px solid var(--divider-soft)',
                    }}
                  >
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
