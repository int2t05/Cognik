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
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '12px 0', fontSize: 14, color: 'var(--text-muted-48)' }}>
      <span>{total > 0 ? `${start}-${end} / ${total} 条` : '0 条'}</span>
      <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
        <PaginationBtn disabled={page <= 1} onClick={() => onChange(page - 1, pageSize)} label="上一页" />
        {Array.from({ length: totalPages }, (_, i) => i + 1)
          .filter((p) => p === 1 || p === totalPages || Math.abs(p - page) <= 1)
          .map((p, i, arr) => (
            <span key={p}>
              {i > 0 && arr[i - 1] !== p - 1 && <span style={{ padding: '0 4px' }}>…</span>}
              <PaginationBtn active={p === page} onClick={() => onChange(p, pageSize)} label={String(p)} />
            </span>
          ))}
        <PaginationBtn disabled={page >= totalPages} onClick={() => onChange(page + 1, pageSize)} label="下一页" />
        <select
          value={pageSize}
          onChange={(e) => onChange(1, Number(e.target.value))}
          style={{
            marginLeft: 12,
            padding: '4px 8px',
            fontSize: 13,
            borderRadius: 'var(--radius-sm)',
            border: '1px solid var(--hairline)',
            background: 'var(--bg-canvas)',
            color: 'var(--text-ink)',
          }}
        >
          {pageSizeOptions.map((s) => (
            <option key={s} value={s}>{s} 条/页</option>
          ))}
        </select>
      </div>
    </div>
  );
}

function PaginationBtn({ active, disabled, onClick, label }: { active?: boolean; disabled?: boolean; onClick: () => void; label: string }) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      style={{
        minWidth: 32,
        height: 32,
        padding: '0 8px',
        fontSize: 13,
        fontWeight: active ? 600 : 400,
        borderRadius: 'var(--radius-sm)',
        border: 'none',
        background: active ? 'var(--bg-pearl)' : 'transparent',
        color: active ? 'var(--accent)' : disabled ? 'var(--text-muted-48)' : 'var(--text-ink)',
        cursor: disabled ? 'default' : 'pointer',
      }}
    >
      {label}
    </button>
  );
}
