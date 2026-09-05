'use client';
import useSWR from 'swr';
import { useTranslations, useLocale } from 'next-intl';
import { getMyTickets } from '@/lib/api/ticket';
import Link from 'next/link';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { IconButton } from '@/components/ui/icon-button';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { EmptyState } from '@/components/shared/EmptyState';
import { InlineError } from '@/components/shared/InlineError';
import { ListSearchInput } from '@/components/shared/ListSearchInput';
import { TableFilterHeader } from '@/components/shared/TableFilterHeader';
import { ticketStatusOptions } from '@/lib/ticket-options';
import { formatDate } from '@/lib/date';
import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { PageTitle } from '@/components/shared/PageTitle';
import { TicketPlus, FileText } from 'lucide-react';

export default function TicketQueryPage() {
  const t = useTranslations();
  const locale = useLocale();
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
    <div className="min-w-0 overflow-hidden">
      <div className="flex justify-between items-center mb-5">
        <PageTitle className="mb-0">{t('ticket.myTitle')}</PageTitle>
        <IconButton label={t('nav.newTicket')} onClick={() => router.push('/portal/tickets/new')}><TicketPlus /></IconButton>
      </div>

      <div className="mb-4">
        <ListSearchInput value={keyword} onDebouncedChange={(v) => { setKeyword(v); setPage(1); }} placeholder={t('ticket.searchPlaceholder')} />
      </div>

      {error && <InlineError />}

      {isEmpty ? (
        <EmptyState
          icon={<FileText size={40} />}
          title={hasFilters ? t('ticket.noMatch') : t('ticket.empty')}
          description={hasFilters ? t('ticket.adjustFilters') : t('ticket.submitFirst')}
          action={hasFilters ? undefined : { label: t('nav.newTicket'), icon: <TicketPlus size={16} />, onClick: () => router.push('/portal/tickets/new') }}
          onClearFilters={hasFilters ? clearFilters : undefined}
        />
      ) : (
        <>
          <DataTable
            columns={[
              { accessorKey: 'ticket_no', meta: { width: '120px' }, header: t('ticket.colNo'), cell: ({ row }) => <span className="font-[var(--font-mono)] text-fine">{row.original.ticket_no}</span> },
              { accessorKey: 'title', header: t('ticket.colTitle'), cell: ({ row }) => <Link href={`/portal/tickets/${row.original.id}`} className="text-[var(--color-accent)]">{row.original.title}</Link> },
              { accessorKey: 'tags', meta: { width: '120px' }, header: t('ticket.colTags'), cell: ({ row }) => (row.original.tags || []).join(', ') || '—' },
              { accessorKey: 'status', meta: { width: '88px' }, header: () => <TableFilterHeader label={t('ticket.colStatus')} value={status} options={ticketStatusOptions(t)} onChange={(v) => { setStatus(v); setPage(1); }} />, cell: ({ row }) => <StatusBadge type="ticket" status={row.original.status} /> },
              { accessorKey: 'created_at', meta: { width: '120px' }, header: t('ticket.colCreatedAt'), cell: ({ row }) => formatDate(row.original.created_at, locale) },
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
