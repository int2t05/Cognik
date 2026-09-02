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
import { formatDate } from '@/lib/date';
import { toast } from 'sonner';
import { EmptyState } from '@/components/shared/EmptyState';
import { InlineError } from '@/components/shared/InlineError';
import { ScrollText } from 'lucide-react';

export default function AuditLogPage() {
  const [page, setPage] = useState(1);
  const { data, error, mutate } = useSWR(`audit-${page}`, () => getAuditLogs({ page, page_size: 10 }));

  const items = data?.items || [];
  const batch = useBatchSelection({
    items,
    batchDeleteFn: batchDeleteAuditLogs,
    onMutate: () => mutate(),
    onError: (msg) => toast.error(msg),
  });

  return (
    <div>
      <div className="flex items-center gap-2 mb-5">
        <PageTitle>审计日志</PageTitle>
        <BatchSelectToolbar selectedCount={batch.selectedIds.size} onDelete={() => batch.setConfirmDelete(true)} onCancel={batch.clearSelection} />
      </div>
      {error && <InlineError />}
      {!error && data?.items?.length === 0 ? (
        <EmptyState icon={<ScrollText size={40} />} title="暂无审计日志" description="系统操作记录将显示在这里" />
      ) : (
        <>
          <DataTable
            columns={[
              { id: '_check', header: () => <BatchSelectHeader items={items} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} onSelectAll={batch.selectAll} />, cell: ({ row }) => <BatchSelectRow row={row.original} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} /> },
              { accessorKey: 'operator_name', header: '操作人' },
              { accessorKey: 'action', header: '操作', cell: ({ row }) => <span className="text-caption">{row.original.action}</span> },
              { accessorKey: 'target_type', header: '对象类型' },
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
