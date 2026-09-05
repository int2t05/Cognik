'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { useTranslations, useLocale } from 'next-intl';
import Link from 'next/link';
import { listAllTickets, batchDeleteTickets, batchCloseTickets } from '@/lib/api/ticket';
import { useBatchSelection } from '@/hooks/useBatchSelection';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { BatchSelectHeader, BatchSelectRow, BatchSelectToolbar } from '@/components/chat/BatchSelectCheckbox';
import { formatDate } from '@/lib/date';
import { FileText, XCircle } from 'lucide-react';
import { toast } from 'sonner';
import { PageTitle } from '@/components/shared/PageTitle';
import { ListSearchInput } from '@/components/shared/ListSearchInput';
import { TableFilterHeader } from '@/components/shared/TableFilterHeader';
import { ticketStatusOptions } from '@/lib/ticket-options';
import { translateError } from '@/lib/api/error';
import { InlineError } from '@/components/shared/InlineError';
import { EmptyState } from '@/components/shared/EmptyState';
import { IconButton } from '@/components/ui/icon-button';

export default function AdminTicketListPage() {
  const t = useTranslations();
  const locale = useLocale();
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState(-1);
  const [keyword, setKeyword] = useState('');
  const { data, error, mutate } = useSWR(`admin-tickets-${page}-${status}-${keyword}`, () => listAllTickets(page, status, keyword), { keepPreviousData: true });

  const items = data?.items || [];

  const batch = useBatchSelection({
    items,
    batchDeleteFn: batchDeleteTickets,
    onMutate: () => mutate(),
    onError: (msg) => toast.error(msg),
  });

  const isEmpty = !error && data && items.length === 0;
  const hasFilters = status !== -1 || keyword !== '';
  const clearFilters = () => { setStatus(-1); setKeyword(''); setPage(1); };
  const [batchCloseConfirm, setBatchCloseConfirm] = useState(false);
  const [batchClosing, setBatchClosing] = useState(false);

  // 批量关闭：逐条返回成功/失败，汇总提示
  const handleBatchClose = async () => {
    setBatchClosing(true);
    try {
      const ids = Array.from(batch.selectedIds).map(Number);
      const res = await batchCloseTickets(ids);
      const ok = res.results.filter((r) => r.success).length;
      const fail = res.results.length - ok;
      toast.success(fail > 0 ? t('ticket.batchClosedSome', { ok, fail }) : t('ticket.batchClosedAll', { ok }));
      setBatchCloseConfirm(false);
      batch.clearSelection();
      mutate();
    } catch (err) {
      toast.error(translateError(err, t, t('ticket.batchCloseFailed')));
    } finally {
      setBatchClosing(false);
    }
  };

  return (
    <div className="min-w-0 overflow-hidden">
      <div className="flex justify-between items-center mb-5">
        <PageTitle>{t('ticket.adminTitle')}</PageTitle>
      </div>
      {error && <InlineError />}
      <div className="mb-4 flex items-center justify-between gap-2 flex-wrap">
        <ListSearchInput value={keyword} onDebouncedChange={(v) => { setKeyword(v); setPage(1); }} placeholder={t('ticket.searchPlaceholder')} />
        <BatchSelectToolbar selectedCount={batch.selectedIds.size} onDelete={() => batch.setConfirmDelete(true)} onCancel={batch.clearSelection} />
        {batch.selectedIds.size > 0 && <IconButton size="sm" variant="ghost" onClick={() => setBatchCloseConfirm(true)}><XCircle size={14} />{t('ticket.batchClose')}</IconButton>}
      </div>
      {isEmpty ? (
        <EmptyState
          icon={<FileText size={40} />}
          title={hasFilters ? t('ticket.noMatch') : t('ticket.adminEmpty')}
          description={hasFilters ? t('ticket.adjustFilters') : t('ticket.adminEmptyDesc')}
          onClearFilters={hasFilters ? clearFilters : undefined}
        />
      ) : (
        <>
          <DataTable
            columns={[
              { id: '_check', meta: { width: '40px' }, header: () => <BatchSelectHeader items={items} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} onSelectAll={batch.selectAll} />, cell: ({ row }) => <BatchSelectRow row={row.original} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} /> },
              { accessorKey: 'ticket_no', meta: { width: '120px' }, header: t('ticket.colNo'), cell: ({ row }) => <span className="font-[var(--font-mono)] text-fine">{row.original.ticket_no}</span> },
              { accessorKey: 'title', header: t('ticket.colTitle'), cell: ({ row }) => <Link href={`/admin/tickets/${row.original.id}`} className="text-[var(--color-accent)]">{row.original.title}</Link> },
              { accessorKey: 'submitter_name', meta: { width: '88px' }, header: t('ticket.colSubmitter') },
              { accessorKey: 'tags', meta: { width: '120px' }, header: t('ticket.colTags'), cell: ({ row }) => (row.original.tags || []).join(', ') || '-' },
              { accessorKey: 'status', meta: { width: '88px' }, header: () => <TableFilterHeader label={t('ticket.colStatus')} value={status} options={ticketStatusOptions(t)} onChange={(v) => { setStatus(v); setPage(1); }} />, cell: ({ row }) => <StatusBadge type="ticket" status={row.original.status} /> },
              { accessorKey: 'created_at', meta: { width: '120px' }, header: t('ticket.colCreatedAt'), cell: ({ row }) => formatDate(row.original.created_at, locale) },
            ]}
            data={items} loading={!data && !error}
          />
          {data && <DataTablePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
        </>
      )}
      <ConfirmDialog open={batch.confirmDelete} onOpenChange={batch.setConfirmDelete}
        title={t('ticket.batchDeleteTitle')}
        message={t('ticket.batchDeleteMessage', { count: batch.selectedIds.size })}
        onConfirm={async () => { await batch.handleBatchDelete(); toast.success(t('ticket.deleted')); }} loading={batch.deleting} danger confirmLabel={t('ticket.delete')} />
      <ConfirmDialog open={batchCloseConfirm} onOpenChange={setBatchCloseConfirm}
        title={t('ticket.batchCloseTitle')}
        message={t('ticket.batchCloseMessage', { count: batch.selectedIds.size })}
        onConfirm={handleBatchClose} loading={batchClosing} confirmLabel={t('ticket.close')} />
    </div>
  );
}
