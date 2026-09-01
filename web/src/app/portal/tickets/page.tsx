'use client';
import useSWR from 'swr';
import { getMyTickets } from '@/lib/api/ticket';
import Link from 'next/link';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { EmptyState } from '@/components/shared/EmptyState';
import { formatDate } from '@/lib/date';
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
        <Button size="icon" aria-label="提交申告" onClick={() => router.push('/portal/tickets/new')}><TicketPlus /></Button>
      </div>

      {error && <p className="text-[var(--color-error)] text-caption mb-4">加载失败，请刷新重试</p>}

      <div className="mb-4">
        <Input className="rounded-[var(--radius-pill)]" placeholder="搜索申告编号或标题..." aria-label="搜索申告" value={keyword} onChange={(e) => { setKeyword(e.target.value); setPage(1); }} />
      </div>

      {isEmpty ? (
        <EmptyState
          icon={<FileText size={40} />}
          title="暂无申告记录"
          description="提交您的第一个运维申告"
          action={{ label: '提交申告', onClick: () => router.push('/portal/tickets/new') }}
        />
      ) : (
        <>
          <DataTable
            columns={[
              { accessorKey: 'ticket_no', header: '编号', cell: ({ row }) => <span className="font-[var(--font-mono)] text-fine">{row.original.ticket_no}</span> },
              { accessorKey: 'title', header: '标题', cell: ({ row }) => <Link href={`/portal/tickets/${row.original.id}`} className="text-[var(--color-accent)]">{row.original.title}</Link> },
              { accessorKey: 'tags', header: '标签', cell: ({ row }) => (row.original.tags || []).join(', ') || '—' },
              { accessorKey: 'status', header: '状态', cell: ({ row }) => <StatusBadge type="ticket" status={row.original.status} /> },
              { accessorKey: 'created_at', header: '提交时间', cell: ({ row }) => formatDate(row.original.created_at) },
            ]}
            data={tickets}
            loading={!data && !error}
          />
          {data && <DataTablePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
        </>
      )}
    </div>
  );
}
