'use client';
import useSWR from 'swr';
import { getMyTickets } from '@/lib/api/ticket';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { useRouter } from 'next/navigation';
import { useState } from 'react';

export default function TicketQueryPage() {
  const [page, setPage] = useState(1);
  const router = useRouter();
  const { data, error } = useSWR(`portal-tickets-${page}`, () => getMyTickets(page));

  return (
    <div>
      <h1 style={{ fontSize: 28, fontWeight: 600, color: 'var(--text-ink)', marginBottom: 24 }}>我的申告</h1>
      {error && <p style={{ color: 'var(--color-error)' }}>加载失败</p>}
      <AppleTable
        columns={[
          { key: 'ticket_no', title: '编号', render: (r) => <span style={{ fontFamily: 'monospace', fontSize: 13 }}>{r.ticket_no}</span> },
          { key: 'title', title: '标题', render: (r) => <a href={`/portal/tickets/${r.id}`} style={{ color: 'var(--accent)' }}>{r.title}</a> },
          { key: 'urgency', title: '紧急程度', render: (r) => ['', '低', '中', '高'][r.urgency] || '—' },
          { key: 'status', title: '状态', render: (r) => <StatusBadge type="ticket" status={r.status} /> },
          { key: 'created_at', title: '提交时间', render: (r) => formatDate(r.created_at) },
        ]}
        data={data?.items || []}
        loading={!data && !error}
        rowKey="id"
      />
      {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
    </div>
  );
}
