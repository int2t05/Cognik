'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { listAllTickets } from '@/lib/api/ticket';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { AppleInput } from '@/components/ui/AppleInput';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { URGENCY_LABELS } from '@/lib/format';
import { ListFilter, Clock, AlertCircle, CheckCircle, XCircle, MessageSquare } from 'lucide-react';
import { useDebounce } from '@/hooks/useDebounce';
import { PageTitle } from '@/components/shared/PageTitle';
import { FilterBar, type FilterOption } from '@/components/shared/FilterBar';

const TICKET_FILTERS: FilterOption<number>[] = [
  { value: -1, label: '全部', icon: <ListFilter size={15} /> },
  { value: 1, label: '待处理', icon: <AlertCircle size={15} /> },
  { value: 2, label: '处理中', icon: <Clock size={15} /> },
  { value: 3, label: '需补充', icon: <MessageSquare size={15} /> },
  { value: 4, label: '已解决', icon: <CheckCircle size={15} /> },
  { value: 5, label: '已关闭', icon: <XCircle size={15} /> },
];

export default function AdminTicketListPage() {
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState(-1);
  const [keyword, setKeyword] = useState('');
  const debouncedKeyword = useDebounce(keyword, 300);
  const { data, error } = useSWR(`admin-tickets-${page}-${status}`, () => listAllTickets(page, status));

  // 客户端关键词过滤
  const items = (data?.items || []).filter((t: { title?: string; ticket_no?: string; submitter_name?: string }) => {
    if (!debouncedKeyword) return true;
    const kw = debouncedKeyword.toLowerCase();
    return (t.title?.toLowerCase().includes(kw)) ||
           (t.ticket_no?.toLowerCase().includes(kw)) ||
           (t.submitter_name?.toLowerCase().includes(kw));
  });

  return (
    <div>
      <PageTitle>申告管理</PageTitle>
      {error && <p className="text-[var(--color-error)] text-caption mb-4">加载失败，请刷新重试</p>}
      <div className="mb-4 flex gap-3 items-center flex-wrap">
        <AppleInput pill placeholder="搜索编号/标题/提交人..." aria-label="搜索申告" value={keyword} onChange={(e) => { setKeyword(e.target.value); setPage(1); }} className="min-w-[240px]" />
        <FilterBar options={TICKET_FILTERS} value={status} onChange={(v) => { setStatus(v); setPage(1); }} />
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
        data={items}
        loading={!data && !error}
        rowKey="id"
      />
      {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
    </div>
  );
}
