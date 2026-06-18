'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { getAuditLogs } from '@/lib/api/audit';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { formatDate } from '@/lib/date';
import { useDebounce } from '@/hooks/useDebounce';

export default function AuditLogPage() {
  const [page, setPage] = useState(1);
  const [filters, setFilters] = useState<Record<string, string | number>>({ page: 1, page_size: 10 });
  const debouncedFilters = useDebounce(filters, 300);
  const { data, error } = useSWR(`audit-${JSON.stringify(debouncedFilters)}`, () => getAuditLogs(debouncedFilters));

  const updateFilter = (k: string, v: string) => { setFilters((prev) => ({ ...prev, [k]: v, page: 1, page_size: 10 })); setPage(1); };

  return (
    <div>
      <h1 className="text-[28px] font-medium text-[var(--color-ink)] mb-6">审计日志</h1>
      <div className="flex gap-3 mb-4 flex-wrap">
        <input placeholder="操作人 ID" type="number" className="h-9 px-3 text-sm rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-40 outline-none focus:border-[var(--color-accent)] focus:shadow-[0_0_0_3px_rgba(0,102,204,0.12)]" onChange={(e) => updateFilter('operator_id', e.target.value)} />
        <input placeholder="操作类型" className="h-9 px-3 text-sm rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-40 outline-none focus:border-[var(--color-accent)] focus:shadow-[0_0_0_3px_rgba(0,102,204,0.12)]" onChange={(e) => updateFilter('action', e.target.value)} />
        <input placeholder="对象类型" className="h-9 px-3 text-sm rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-40 outline-none focus:border-[var(--color-accent)] focus:shadow-[0_0_0_3px_rgba(0,102,204,0.12)]" onChange={(e) => updateFilter('target_type', e.target.value)} />
        <input placeholder="起始日期" className="h-9 px-3 text-sm rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-40 outline-none focus:border-[var(--color-accent)] focus:shadow-[0_0_0_3px_rgba(0,102,204,0.12)]" onChange={(e) => updateFilter('date_from', e.target.value)} />
        <input placeholder="结束日期" className="h-9 px-3 text-sm rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-40 outline-none focus:border-[var(--color-accent)] focus:shadow-[0_0_0_3px_rgba(0,102,204,0.12)]" onChange={(e) => updateFilter('date_to', e.target.value)} />
      </div>
      <AppleTable
        columns={[
          { key: 'operator_name', title: '操作人' },
          { key: 'action', title: '操作', render: (r) => <span className="text-[13px]">{r.action}</span> },
          { key: 'target_type', title: '对象类型' },
          { key: 'ip_address', title: 'IP' },
          { key: 'created_at', title: '时间', render: (r) => formatDate(r.created_at) },
        ]}
        data={data?.items || []} loading={!data && !error} rowKey="id"
      />
      {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => { setPage(p); setFilters((prev) => ({ ...prev, page: p })); }} />}
    </div>
  );
}
