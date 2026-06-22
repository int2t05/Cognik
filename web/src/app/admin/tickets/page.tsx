'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { listAllTickets } from '@/lib/api/ticket';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { URGENCY_LABELS } from '@/lib/format';
import { ListFilter, Clock, AlertCircle, CheckCircle, XCircle, MessageSquare } from 'lucide-react';

const FILTERS = [
  { v: -1, l: '全部申告', icon: <ListFilter size={13} /> },
  { v: 1, l: '待处理', icon: <AlertCircle size={13} /> },
  { v: 2, l: '处理中', icon: <Clock size={13} /> },
  { v: 3, l: '需补充信息', icon: <MessageSquare size={13} /> },
  { v: 4, l: '已解决', icon: <CheckCircle size={13} /> },
  { v: 5, l: '已关闭', icon: <XCircle size={13} /> },
];

export default function AdminTicketListPage() {
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState(-1);
  const { data, error } = useSWR(`admin-tickets-${page}-${status}`, () => listAllTickets(page, status));

  return (
    <div>
      <h1 className="text-hero font-semibold text-[var(--color-ink)] mb-6">申告管理</h1>
      <div className="mb-4 flex gap-2 flex-wrap">
        {FILTERS.map((o) => (
          <button
            key={o.v}
            onClick={() => { setStatus(o.v); setPage(1); }}
            className={`inline-flex items-center gap-1.5 px-3.5 py-1.5 border rounded-[var(--radius-pill)] text-caption cursor-pointer transition ${
              status === o.v
                ? 'bg-[var(--color-accent)] border-[var(--color-accent)] text-[var(--color-on-accent)] font-semibold'
                : 'bg-[var(--color-pearl)] border-[var(--color-divider-soft)] text-[var(--color-text-muted-80)] hover:border-[var(--color-hairline)]'
            }`}
          >
            {o.icon}
            {o.l}
          </button>
        ))}
      </div>
      <AppleTable
        columns={[
          { key: 'ticket_no', title: '编号', render: (r) => <span className="font-[var(--font-mono)] text-fine">{r.ticket_no}</span> },
          { key: 'title', title: '标题', render: (r) => <a href={`/admin/tickets/${r.id}`} className="text-[var(--color-accent)]">{r.title}</a> },
          { key: 'submitter_name', title: '提交人' },
          { key: 'urgency', title: '紧急程度', render: (r) => URGENCY_LABELS[r.urgency] },
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
