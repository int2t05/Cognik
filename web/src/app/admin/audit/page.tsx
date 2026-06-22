'use client';
import useSWR from 'swr';
import { useState, useId } from 'react';
import { getAuditLogs } from '@/lib/api/audit';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { formatDate } from '@/lib/date';
import { useDebounce } from '@/hooks/useDebounce';

export default function AuditLogPage() {
  const [params, setParams] = useState<Record<string, string | number>>({ page: 1, page_size: 10 });
  const debouncedParams = useDebounce(params, 300);
  const { data, error } = useSWR(`audit-${JSON.stringify(debouncedParams)}`, () => getAuditLogs(debouncedParams));
  const idOp = useId(); const idAct = useId(); const idType = useId(); const idFrom = useId(); const idTo = useId();

  const updateParam = (k: string, v: string) => setParams((prev) => ({ ...prev, [k]: v, page: 1, page_size: 10 }));
  const changePage = (p: number) => setParams((prev) => ({ ...prev, page: p }));

  return (
    <div>
      <h1 className="text-hero font-semibold text-[var(--color-ink)] mb-6">审计日志</h1>
      <div className="flex gap-3 mb-4 flex-wrap items-end">
        <div>
          <label htmlFor={idOp} className="block text-caption text-[var(--color-text-muted-48)] mb-1">操作人 ID</label>
          <input id={idOp} placeholder="操作人 ID" type="number" className="h-9 px-3 text-caption rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-40 outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" onChange={(e) => updateParam('operator_id', e.target.value)} />
        </div>
        <div>
          <label htmlFor={idAct} className="block text-caption text-[var(--color-text-muted-48)] mb-1">操作类型</label>
          <input id={idAct} placeholder="操作类型" className="h-9 px-3 text-caption rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-40 outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" onChange={(e) => updateParam('action', e.target.value)} />
        </div>
        <div>
          <label htmlFor={idType} className="block text-caption text-[var(--color-text-muted-48)] mb-1">对象类型</label>
          <input id={idType} placeholder="对象类型" className="h-9 px-3 text-caption rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-40 outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" onChange={(e) => updateParam('target_type', e.target.value)} />
        </div>
        <div>
          <label htmlFor={idFrom} className="block text-caption text-[var(--color-text-muted-48)] mb-1">开始日期</label>
          <input id={idFrom} type="date" className="h-9 px-3 text-caption rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-40 outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" onChange={(e) => updateParam('date_from', e.target.value)} />
        </div>
        <div>
          <label htmlFor={idTo} className="block text-caption text-[var(--color-text-muted-48)] mb-1">结束日期</label>
          <input id={idTo} type="date" className="h-9 px-3 text-caption rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-40 outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" onChange={(e) => updateParam('date_to', e.target.value)} />
        </div>
      </div>
      {error && <p className="text-[var(--color-error)] text-caption mb-4">加载失败，请刷新重试</p>}
      <AppleTable
        columns={[
          { key: 'operator_name', title: '操作人' },
          { key: 'action', title: '操作', render: (r) => <span className="text-caption">{r.action}</span> },
          { key: 'target_type', title: '对象类型' },
          { key: 'ip_address', title: 'IP' },
          { key: 'created_at', title: '时间', render: (r) => formatDate(r.created_at) },
        ]}
        data={data?.items || []} loading={!data && !error} rowKey="id"
      />
      {data && <ApplePagination page={Number(params.page)} pageSize={10} total={data.total} onChange={changePage} />}
    </div>
  );
}
