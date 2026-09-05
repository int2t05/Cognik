'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { useTranslations, useLocale } from 'next-intl';
import { getAuditLogs, batchDeleteAuditLogs } from '@/lib/api/audit';
import { useBatchSelection } from '@/hooks/useBatchSelection';
import { PageTitle } from '@/components/shared/PageTitle';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { BatchSelectHeader, BatchSelectRow, BatchSelectToolbar } from '@/components/chat/BatchSelectCheckbox';
import { ListSearchInput } from '@/components/shared/ListSearchInput';
import { TableFilterHeader, type TableFilterOption } from '@/components/shared/TableFilterHeader';
import { formatDate } from '@/lib/date';
import { toast } from 'sonner';
import { EmptyState } from '@/components/shared/EmptyState';
import { InlineError } from '@/components/shared/InlineError';
import { ScrollText } from 'lucide-react';

/** 审计对象类型 → i18n 键；未知类型原样显示 */
const AUDIT_TYPE_KEYS: Record<string, string> = {
  user: 'audit.typeUser',
  role: 'audit.typeRole',
  knowledge_article: 'audit.typeArticle',
  knowledge_base: 'audit.typeKb',
  ticket: 'audit.typeTicket',
  config: 'audit.typeConfig',
  llm_config: 'audit.typeLlm',
};

export default function AuditLogPage() {
  const t = useTranslations();
  const locale = useLocale();
  const [page, setPage] = useState(1);
  const [keyword, setKeyword] = useState('');
  const [targetType, setTargetType] = useState('');
  const { data, error, mutate } = useSWR(`audit-${page}-${keyword}-${targetType}`, () => getAuditLogs({ page, page_size: 10, keyword, target_type: targetType }), { keepPreviousData: true });

  const items = data?.items || [];
  const batch = useBatchSelection({
    items,
    batchDeleteFn: batchDeleteAuditLogs,
    onMutate: () => mutate(),
    onError: (msg) => toast.error(msg),
  });

  const typeLabel = (key: string) => AUDIT_TYPE_KEYS[key] ? t(AUDIT_TYPE_KEYS[key]) : key;
  const typeOptions: TableFilterOption<string>[] = [
    { value: '', label: t('common.all') },
    ...Object.keys(AUDIT_TYPE_KEYS).map((value) => ({ value, label: typeLabel(value) })),
  ];

  const isEmpty = !error && data?.items?.length === 0;
  const hasFilters = targetType !== '' || keyword !== '';
  const clearFilters = () => { setTargetType(''); setKeyword(''); setPage(1); };

  return (
    <div className="min-w-0 overflow-hidden">
      <div className="flex items-center gap-2 mb-5">
        <PageTitle className="mb-0">{t('audit.title')}</PageTitle>
        <ListSearchInput value={keyword} onDebouncedChange={(v) => { setKeyword(v); setPage(1); }} placeholder={t('audit.searchPlaceholder')} />
        <BatchSelectToolbar selectedCount={batch.selectedIds.size} onDelete={() => batch.setConfirmDelete(true)} onCancel={batch.clearSelection} />
      </div>
      {error && <InlineError />}
      {!error && data?.items?.length === 0 ? (
        <EmptyState icon={<ScrollText size={40} />} title={hasFilters ? t('audit.noMatch') : t('audit.empty')} description={hasFilters ? t('ticket.adjustFilters') : t('audit.emptyDesc')} onClearFilters={hasFilters ? clearFilters : undefined} />
      ) : (
        <>
          <DataTable
            columns={[
              { id: '_check', meta: { width: '40px' }, header: () => <BatchSelectHeader items={items} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} onSelectAll={batch.selectAll} />, cell: ({ row }) => <BatchSelectRow row={row.original} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} /> },
              { accessorKey: 'operator_name', meta: { width: '88px' }, header: t('audit.colOperator') },
              { accessorKey: 'action', meta: { width: '100px' }, header: t('common.actions'), cell: ({ row }) => <span className="text-caption">{row.original.action}</span> },
              { accessorKey: 'target_type', meta: { width: '96px' }, header: () => <TableFilterHeader label={t('audit.colTargetType')} value={targetType} options={typeOptions} onChange={(v) => { setTargetType(v); setPage(1); }} />, cell: ({ row }) => <span className="text-caption">{typeLabel(row.original.target_type)}</span> },
              { accessorKey: 'ip_address', meta: { width: '120px' }, header: t('audit.colIp') },
              { accessorKey: 'created_at', meta: { width: '120px' }, header: t('audit.colTime'), cell: ({ row }) => formatDate(row.original.created_at, locale) },
            ]}
            data={items} loading={!data && !error}
          />
          {data && <DataTablePagination page={page} pageSize={10} total={data.total} onChange={setPage} />}
        </>
      )}
      <ConfirmDialog open={batch.confirmDelete} onOpenChange={batch.setConfirmDelete}
        title={t('audit.batchDeleteTitle')}
        message={t('audit.batchDeleteMessage', { count: batch.selectedIds.size })}
        onConfirm={async () => { await batch.handleBatchDelete(); toast.success(t('common.deleted')); }} loading={batch.deleting} danger confirmLabel={t('common.delete')} />
    </div>
  );
}
