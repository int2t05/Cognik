/** AppleTable — 无边框 + 行悬浮 + loading skeleton + empty state */
'use client';

import { type ReactNode } from 'react';
import { AppleSpinner } from './AppleSpinner';
import styles from './AppleTable.module.css';

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
    <div className={styles.wrapper}>
      <table className={styles.table}>
        <thead>
          <tr>
            {columns.map((col) => (
              <th
                key={col.key}
                className={styles.th}
                style={{ width: col.width }}
              >
                {col.title}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {loading ? (
            <tr>
              <td colSpan={columns.length} className={styles.loading}>
                <AppleSpinner />
              </td>
            </tr>
          ) : data.length === 0 ? (
            <tr>
              <td colSpan={columns.length} className={styles.empty}>
                {emptyText}
              </td>
            </tr>
          ) : (
            data.map((row) => (
              <tr key={getKey(row)} className={styles.row}>
                {columns.map((col) => (
                  <td key={col.key} className={styles.td}>
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
