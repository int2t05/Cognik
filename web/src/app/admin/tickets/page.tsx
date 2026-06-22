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
  { v: -1, label: '全部申告', icon: <ListFilter size={17} /> },
  { v: 1, label: '待处理', icon: <AlertCircle size={17} /> },
  { v: 2, label: '处理中', icon: <Clock size={17} /> },
  { v: 3, label: '需补充信息', icon: <MessageSquare size={17} /> },
  { v: 4, label: '已解决', icon: <CheckCircle size={17} /> },
  { v: 5, label: '已关闭', icon: <XCircle size={17} /> },
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
            aria-label={o.label}
            className={`p-2 border rounded-[var(--radius-pill)] cursor-pointer transition ${
              status === o.v
                ? 'bg-[var(--color-accent)] border-[var(--color-accent)] text-[var(--color-on-accent)]'
                : 'bg-[var(--color-pearl)] border-[var(--color-divider-soft)] text-[var(--color-text-muted-80)] hover:border-[var(--color-hairline)]'
            }`}
          >
            {o.icon}
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
