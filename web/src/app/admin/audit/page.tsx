'use client';
import useSWR from 'swr';
import { useState } from 'react';
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

const AUDIT_TARGET_TYPE_OPTIONS: TableFilterOption<string>[] = [
  { value: '', label: '全部' },
  { value: 'user', label: '用户' },
  { value: 'role', label: '角色' },
  { value: 'knowledge_article', label: '知识文章' },
  { value: 'knowledge_base', label: '知识库' },
  { value: 'ticket', label: '申告' },
  { value: 'config', label: '系统配置' },
  { value: 'llm_config', label: 'LLM 配置' },
];

export default function AuditLogPage() {
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

  const isEmpty = !error && data?.items?.length === 0;
  const hasFilters = targetType !== '' || keyword !== '';
  const clearFilters = () => { setTargetType(''); setKeyword(''); setPage(1); };

  return (
    <div>
      <div className="flex items-center gap-2 mb-5">
        <PageTitle className="mb-0">审计日志</PageTitle>
        <ListSearchInput value={keyword} onDebouncedChange={(v) => { setKeyword(v); setPage(1); }} placeholder="搜索操作/对象…" />
        <BatchSelectToolbar selectedCount={batch.selectedIds.size} onDelete={() => batch.setConfirmDelete(true)} onCancel={batch.clearSelection} />
      </div>
      {error && <InlineError />}
      {!error && data?.items?.length === 0 ? (
        <EmptyState icon={<ScrollText size={40} />} title={hasFilters ? '未找到匹配的审计日志' : '暂无审计日志'} description={hasFilters ? '尝试调整筛选条件或清除筛选' : '系统操作记录将显示在这里'} onClearFilters={hasFilters ? clearFilters : undefined} />
      ) : (
        <>
          <DataTable
            columns={[
              { id: '_check', header: () => <BatchSelectHeader items={items} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} onSelectAll={batch.selectAll} />, cell: ({ row }) => <BatchSelectRow row={row.original} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} /> },
              { accessorKey: 'operator_name', header: '操作人' },
              { accessorKey: 'action', header: '操作', cell: ({ row }) => <span className="text-caption">{row.original.action}</span> },
              { accessorKey: 'target_type', header: () => <TableFilterHeader label="对象类型" value={targetType} options={AUDIT_TARGET_TYPE_OPTIONS} onChange={(v) => { setTargetType(v); setPage(1); }} /> },
              { accessorKey: 'ip_address', header: 'IP' },
              { accessorKey: 'created_at', header: '时间', cell: ({ row }) => formatDate(row.original.created_at) },
            ]}
            data={items} loading={!data && !error}
          />
          {data && <DataTablePagination page={page} pageSize={10} total={data.total} onChange={setPage} />}
        </>
      )}
      <ConfirmDialog open={batch.confirmDelete} onOpenChange={batch.setConfirmDelete}
        title="批量删除审计日志"
        message={`确定要删除 ${batch.selectedIds.size} 条审计日志吗？此操作不可撤销。`}
        onConfirm={async () => { await batch.handleBatchDelete(); toast.success('已删除'); }} loading={batch.deleting} danger confirmLabel="删除" />
    </div>
  );
}
