/** ApplePagination — 精简紧凑样式 */
'use client';

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
    <div className="flex items-center justify-center gap-1 py-4">
      <span className="text-caption text-[var(--color-text-muted-48)]">
        {total > 0 ? `${start}-${end} / ${total} 条` : '0 条'}
      </span>
      <div className="flex items-center gap-1">
        <PaginationBtn
          disabled={page <= 1}
          onClick={() => onChange(page - 1, pageSize)}
          label="上一页"
        />
        {Array.from({ length: totalPages }, (_, i) => i + 1)
          .filter((p) => p === 1 || p === totalPages || Math.abs(p - page) <= 1)
          .map((p, i, arr) => (
            <span key={p}>
              {i > 0 && arr[i - 1] !== p - 1 && (
                <span className="px-1 text-[var(--color-text-muted-48)]">...</span>
              )}
              <PaginationBtn
                active={p === page}
                onClick={() => onChange(p, pageSize)}
                label={String(p)}
              />
            </span>
          ))}
        <PaginationBtn
          disabled={page >= totalPages}
          onClick={() => onChange(page + 1, pageSize)}
          label="下一页"
        />
      </div>
      <select
        aria-label="每页条数"
        className="ml-3 px-2 py-1 text-caption rounded-lg border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] font-sans outline-none"
        value={pageSize}
        onChange={(e) => onChange(1, Number(e.target.value))}
      >
        {pageSizeOptions.map((s) => (
          <option key={s} value={s}>
            {s} 条/页
          </option>
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
}: {
  active?: boolean;
  disabled?: boolean;
  onClick: () => void;
  label: string;
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={`min-w-[36px] h-9 flex items-center justify-center text-caption rounded-lg border-0 font-sans cursor-pointer transition hover:bg-[var(--color-divider-soft)] ${
        active
          ? 'bg-[var(--color-accent)] text-[var(--color-on-accent)] hover:bg-[var(--color-accent-hover)]'
          : 'bg-transparent text-[var(--color-ink)]'
      } ${disabled ? 'opacity-30 cursor-default' : ''}`}
    >
      {label}
    </button>
  );
}
