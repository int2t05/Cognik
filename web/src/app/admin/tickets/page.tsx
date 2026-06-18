'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { listAllTickets } from '@/lib/api/ticket';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';

export default function AdminTicketListPage() {
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState(-1);
  const { data, error } = useSWR(`admin-tickets-${page}-${status}`, () => listAllTickets(page, status));

  return (
    <div>
      <h1 style={{ fontSize: 28, fontWeight: 600, color: 'var(--text-ink)', marginBottom: 24 }}>申告管理</h1>
      <div style={{ marginBottom: 16, display: 'flex', gap: 8 }}>
        {[{ v: -1, l: '全部' }, { v: 1, l: '待处理' }, { v: 2, l: '处理中' }, { v: 3, l: '需补充' }, { v: 4, l: '已解决' }, { v: 5, l: '已关闭' }].map((o) => (
          <button key={o.v} onClick={() => { setStatus(o.v); setPage(1); }}
            style={{ padding: '6px 14px', border: 'none', borderRadius: 'var(--radius-pill)', background: status === o.v ? 'var(--accent)' : 'var(--divider-soft)', color: status === o.v ? '#fff' : 'var(--text-ink)', fontSize: 13, cursor: 'pointer', fontWeight: status === o.v ? 600 : 400 }}>
            {o.l}
          </button>
        ))}
      </div>
      <AppleTable
        columns={[
          { key: 'ticket_no', title: '编号', render: (r) => <span style={{ fontFamily: 'monospace', fontSize: 12 }}>{r.ticket_no}</span> },
          { key: 'title', title: '标题', render: (r) => <a href={`/admin/tickets/${r.id}`} style={{ color: 'var(--accent)' }}>{r.title}</a> },
          { key: 'submitter_name', title: '提交人' },
          { key: 'urgency', title: '紧急', render: (r) => ['', '低', '中', '高'][r.urgency] },
          { key: 'status', title: '状态', render: (r) => <StatusBadge type="ticket" status={r.status} /> },
          { key: 'created_at', title: '时间', render: (r) => formatDate(r.created_at) },
        ]}
        data={data?.items || []}
        loading={!data && !error}
        rowKey="id"
      />
      {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
    </div>
  );
}
