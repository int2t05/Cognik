'use client';
import useSWR from 'swr';
import { getMyTickets } from '@/lib/api/ticket';
import Link from 'next/link';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { Button } from '@/components/ui/button';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { EmptyState } from '@/components/shared/EmptyState';
import { InlineError } from '@/components/shared/InlineError';
import { ListSearchInput } from '@/components/shared/ListSearchInput';
import { formatDate } from '@/lib/date';
import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { PageTitle } from '@/components/shared/PageTitle';
import { TicketPlus, FileText } from 'lucide-react';

export default function TicketQueryPage() {
  const [page, setPage] = useState(1);
  const [keyword, setKeyword] = useState('');
  const router = useRouter();
  const { data, error } = useSWR(`portal-tickets-${page}-${keyword}`, () => getMyTickets(page, keyword));

  const tickets = data?.items ?? [];
  const isEmpty = !error && data && tickets.length === 0;

  return (
    <div>
      <div className="flex justify-between items-center mb-5">
        <PageTitle className="mb-0">我的申告</PageTitle>
        <Button size="icon" aria-label="提交申告" onClick={() => router.push('/portal/tickets/new')}><TicketPlus /></Button>
      </div>

      <div className="mb-4">
        <ListSearchInput value={keyword} onDebouncedChange={(v) => { setKeyword(v); setPage(1); }} placeholder="搜索标题、编号、描述…" />
      </div>

      {error && <InlineError />}

      {isEmpty ? (
        <EmptyState
          icon={<FileText size={40} />}
          title={keyword ? '未找到匹配的申告' : '暂无申告记录'}
          description={keyword ? `没有标题或描述含"${keyword}"的申告` : '提交您的第一个运维申告'}
          action={keyword ? undefined : { label: '提交申告', onClick: () => router.push('/portal/tickets/new') }}
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
