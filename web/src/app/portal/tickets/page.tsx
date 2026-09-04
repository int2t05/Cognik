'use client';
import useSWR from 'swr';
import { getMyTickets } from '@/lib/api/ticket';
import Link from 'next/link';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { IconButton } from '@/components/ui/icon-button';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { EmptyState } from '@/components/shared/EmptyState';
import { InlineError } from '@/components/shared/InlineError';
import { ListSearchInput } from '@/components/shared/ListSearchInput';
import { TableFilterHeader, type TableFilterOption } from '@/components/shared/TableFilterHeader';
import { formatDate } from '@/lib/date';
import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { PageTitle } from '@/components/shared/PageTitle';
import { TicketPlus, FileText } from 'lucide-react';

const TICKET_STATUS_OPTIONS: TableFilterOption<number>[] = [
  { value: -1, label: '全部' },
  { value: 1, label: '待处理' },
  { value: 2, label: '处理中' },
  { value: 3, label: '需补充' },
  { value: 4, label: '已解决' },
  { value: 5, label: '已关闭' },
];

export default function TicketQueryPage() {
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState(-1);
  const [keyword, setKeyword] = useState('');
  const router = useRouter();
  const { data, error } = useSWR(`portal-tickets-${page}-${status}-${keyword}`, () => getMyTickets(page, status, keyword), { keepPreviousData: true });

  const tickets = data?.items ?? [];
  const isEmpty = !error && data && tickets.length === 0;
  const hasFilters = status !== -1 || keyword !== '';
  const clearFilters = () => { setStatus(-1); setKeyword(''); setPage(1); };

  return (
    <div>
      <div className="flex justify-between items-center mb-5">
        <PageTitle className="mb-0">我的申告</PageTitle>
        <IconButton label="提交申告" onClick={() => router.push('/portal/tickets/new')}><TicketPlus /></IconButton>
      </div>

      <div className="mb-4">
        <ListSearchInput value={keyword} onDebouncedChange={(v) => { setKeyword(v); setPage(1); }} placeholder="搜索标题、编号、描述…" />
      </div>

      {error && <InlineError />}

      {isEmpty ? (
        <EmptyState
          icon={<FileText size={40} />}
          title={hasFilters ? '未找到匹配的申告' : '暂无申告记录'}
          description={hasFilters ? '尝试调整筛选条件或清除筛选' : '提交您的第一个运维申告'}
          action={hasFilters ? undefined : { label: '提交申告', icon: <TicketPlus size={16} />, onClick: () => router.push('/portal/tickets/new') }}
          onClearFilters={hasFilters ? clearFilters : undefined}
        />
      ) : (
        <>
          <DataTable
            columns={[
              { accessorKey: 'ticket_no', meta: { width: '120px' }, header: '编号', cell: ({ row }) => <span className="font-[var(--font-mono)] text-fine">{row.original.ticket_no}</span> },
              { accessorKey: 'title', header: '标题', cell: ({ row }) => <Link href={`/portal/tickets/${row.original.id}`} className="text-[var(--color-accent)]">{row.original.title}</Link> },
              { accessorKey: 'tags', meta: { width: '120px' }, header: '标签', cell: ({ row }) => (row.original.tags || []).join(', ') || '—' },
              { accessorKey: 'status', meta: { width: '88px' }, header: () => <TableFilterHeader label="状态" value={status} options={TICKET_STATUS_OPTIONS} onChange={(v) => { setStatus(v); setPage(1); }} />, cell: ({ row }) => <StatusBadge type="ticket" status={row.original.status} /> },
              { accessorKey: 'created_at', meta: { width: '120px' }, header: '提交时间', cell: ({ row }) => formatDate(row.original.created_at) },
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
