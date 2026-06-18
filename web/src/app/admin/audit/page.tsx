'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { getAuditLogs } from '@/lib/api/audit';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { formatDate } from '@/lib/date';

export default function AuditLogPage() {
  const [page, setPage] = useState(1);
  const [filters, setFilters] = useState<Record<string, string | number>>({ page: 1, page_size: 10 });
  const { data, error } = useSWR(`audit-${JSON.stringify(filters)}`, () => getAuditLogs(filters));

  const updateFilter = (k: string, v: string) => { const next = { ...filters, [k]: v, page: 1, page_size: 10 }; setFilters(next); setPage(1); };

  return (
    <div>
      <h1 style={{ fontSize: 28, fontWeight: 600, color: 'var(--text-ink)', marginBottom: 24 }}>审计日志</h1>
      <div style={{ display: 'flex', gap: 12, marginBottom: 16, flexWrap: 'wrap' }}>
        <input placeholder="操作人 ID" type="number" style={filterInputStyle} onChange={(e) => updateFilter('operator_id', e.target.value)} />
        <input placeholder="操作类型" style={filterInputStyle} onChange={(e) => updateFilter('action', e.target.value)} />
        <input placeholder="对象类型" style={filterInputStyle} onChange={(e) => updateFilter('target_type', e.target.value)} />
        <input placeholder="起始日期 YYYY-MM-DD" style={filterInputStyle} onChange={(e) => updateFilter('date_from', e.target.value)} />
        <input placeholder="结束日期 YYYY-MM-DD" style={filterInputStyle} onChange={(e) => updateFilter('date_to', e.target.value)} />
      </div>
      <AppleTable
        columns={[
          { key: 'operator_name', title: '操作人' },
          { key: 'action', title: '操作', render: (r) => <span style={{ fontSize: 13 }}>{r.action}</span> },
          { key: 'target_type', title: '对象类型' },
          { key: 'ip_address', title: 'IP' },
          { key: 'created_at', title: '时间', render: (r) => formatDate(r.created_at) },
        ]}
        data={data?.items || []}
        loading={!data && !error}
        rowKey="id"
      />
      {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => { setPage(p); setFilters({ ...filters, page: p }); }} />}
    </div>
  );
}

const filterInputStyle: React.CSSProperties = {
  height: 36, padding: '0 12px', fontSize: 14, borderRadius: 'var(--radius-sm)',
  border: '1px solid var(--hairline)', background: 'var(--bg-canvas)', color: 'var(--text-ink)', width: 160,
};
