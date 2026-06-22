'use client';
import useSWR from 'swr';
import { getMyTickets } from '@/lib/api/ticket';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput } from '@/components/ui/AppleInput';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { URGENCY_LABELS } from '@/lib/format';
import { useRouter } from 'next/navigation';
import { useState, useMemo } from 'react';
import { PageTitle } from '@/components/shared/PageTitle';
import { TicketPlus, FileText } from 'lucide-react';
import { useDebounce } from '@/hooks/useDebounce';

export default function TicketQueryPage() {
  const [page, setPage] = useState(1);
  const [keyword, setKeyword] = useState('');
  const debouncedKeyword = useDebounce(keyword, 300);
  const router = useRouter();
  const { data, error } = useSWR(`portal-tickets-${page}`, () => getMyTickets(page));

  const tickets = useMemo(() => {
    const items = data?.items ?? [];
    if (!debouncedKeyword) return items;
    const kw = debouncedKeyword.toLowerCase();
    return items.filter((t: { title?: string; ticket_no?: string }) =>
      (t.title?.toLowerCase().includes(kw)) ||
      (t.ticket_no?.toLowerCase().includes(kw))
    );
  }, [data, debouncedKeyword]);
  const isEmpty = !error && data && tickets.length === 0;

  return (
    <div>
      <div className="flex justify-between items-center mb-5">
        <PageTitle>我的申告</PageTitle>
        <AppleButton onClick={() => router.push('/portal/tickets/new')} className="p-3.5" aria-label="提交申告">
          <TicketPlus size={18} />
        </AppleButton>
      </div>

      {error && <p className="text-[var(--color-error)] text-caption mb-4">加载失败，请刷新重试</p>}

      <div className="mb-4">
        <AppleInput pill placeholder="搜索申告编号或标题..." aria-label="搜索申告" value={keyword} onChange={(e) => { setKeyword(e.target.value); setPage(1); }} />
      </div>

      {isEmpty ? (
        <div className="text-center py-16">
          <FileText size={32} className="mx-auto mb-4 text-[var(--color-text-muted-48)]" />
          <p className="text-title text-[var(--color-text-muted-48)] mb-2">暂无申告记录</p>
          <p className="text-caption text-[var(--color-text-muted-48)] mb-5">遇到问题？提交申告让运维人员帮您处理</p>
          <AppleButton variant="ghost" onClick={() => router.push('/portal/tickets/new')}><TicketPlus size={18} /> 提交申告</AppleButton>
        </div>
      ) : (
        <>
          <AppleTable
            columns={[
              { key: 'ticket_no', title: '编号', render: (r) => <span className="font-[var(--font-mono)] text-fine">{r.ticket_no}</span> },
              { key: 'title', title: '标题', render: (r) => <a href={`/portal/tickets/${r.id}`} className="text-[var(--color-accent)]">{r.title}</a> },
              { key: 'urgency', title: '紧急程度', render: (r) => URGENCY_LABELS[r.urgency] || '—' },
              { key: 'status', title: '状态', render: (r) => <StatusBadge type="ticket" status={r.status} /> },
              { key: 'created_at', title: '提交时间', render: (r) => formatDate(r.created_at) },
            ]}
            data={tickets}
            loading={!data && !error}
            rowKey="id"
          />
          {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
        </>
      )}
    </div>
  );
}
