/** ApplePagination — 精简紧凑样式 */
'use client';

import styles from './ApplePagination.module.css';

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
  const start = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const end = Math.min(page * pageSize, total);

  return (
    <div className={styles.pagination}>
      <span className={styles.info}>
        {total > 0 ? `${start}-${end} / ${total} 条` : '0 条'}
      </span>
      <div className={styles.actions}>
        <PaginationBtn
          disabled={page <= 1}
          onClick={() => onChange(page - 1, pageSize)}
          label="上一页"
          styles={styles}
        />
        {Array.from({ length: totalPages }, (_, i) => i + 1)
          .filter((p) => p === 1 || p === totalPages || Math.abs(p - page) <= 1)
          .map((p, i, arr) => (
            <span key={p}>
              {i > 0 && arr[i - 1] !== p - 1 && <span className={styles.ellipsis}>...</span>}
              <PaginationBtn
                active={p === page}
                onClick={() => onChange(p, pageSize)}
                label={String(p)}
                styles={styles}
              />
            </span>
          ))}
        <PaginationBtn
          disabled={page >= totalPages}
          onClick={() => onChange(page + 1, pageSize)}
          label="下一页"
          styles={styles}
        />
      </div>
      <select
        className={styles.select}
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

function PaginationBtn({
  active,
  disabled,
  onClick,
  label,
  styles: css,
}: {
  active?: boolean;
  disabled?: boolean;
  onClick: () => void;
  label: string;
  styles: Record<string, string>;
}) {
  const classNames = [css.btn, active ? css.active : '', disabled ? css.disabled : '']
    .filter(Boolean).join(' ');

  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={classNames}
    >
      {label}
    </button>
  );
}
