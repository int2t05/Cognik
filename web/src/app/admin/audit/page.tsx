'use client';
import useSWR from 'swr';
import { useState, useId } from 'react';
import { getAuditLogs, batchDeleteAuditLogs } from '@/lib/api/audit';
import { useBatchSelection } from '@/hooks/useBatchSelection';
import { PageTitle } from '@/components/shared/PageTitle';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { BatchSelectHeader, BatchSelectRow, BatchSelectToolbar } from '@/components/chat/BatchSelectCheckbox';
import { formatDate } from '@/lib/date';
import { useDebounce } from '@/hooks/useDebounce';
import { useToast } from '@/hooks/useToast';
import { EmptyState } from '@/components/shared/EmptyState';
import { ScrollText } from 'lucide-react';

export default function AuditLogPage() {
  const [params, setParams] = useState<Record<string, string | number>>({ page: 1, page_size: 10 });
  const debouncedParams = useDebounce(params, 300);
  const { data, error, mutate } = useSWR(`audit-${JSON.stringify(debouncedParams)}`, () => getAuditLogs(debouncedParams));
  const toast = useToast();
  const idOp = useId(); const idAct = useId(); const idType = useId(); const idFrom = useId(); const idTo = useId();

  const items = data?.items || [];
  const batch = useBatchSelection({
    items,
    batchDeleteFn: batchDeleteAuditLogs,
    onMutate: () => mutate(),
    onError: (msg) => toast.error(msg),
  });

  const updateParam = (k: string, v: string) => setParams((prev) => ({ ...prev, [k]: v, page: 1, page_size: 10 }));
  const changePage = (p: number) => setParams((prev) => ({ ...prev, page: p }));

  return (
    <div>
      <div className="flex items-center gap-2 mb-4">
        <PageTitle>审计日志</PageTitle>
        <BatchSelectToolbar selectedCount={batch.selectedIds.size} onDelete={() => batch.setConfirmDelete(true)} onCancel={batch.clearSelection} />
      </div>
      <div className="flex gap-2 mb-4 flex-wrap items-end">
        <div><label htmlFor={idOp} className="block text-fine text-[var(--color-text-muted-48)] mb-0.5 pl-2">操作人</label>
          <input id={idOp} placeholder="ID" type="number" className="h-8 px-2.5 text-fine rounded-[var(--radius-md)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-24 outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" onChange={(e) => updateParam('operator_id', e.target.value)} /></div>
        <div><label htmlFor={idAct} className="block text-fine text-[var(--color-text-muted-48)] mb-0.5 pl-2">操作</label>
          <input id={idAct} placeholder="类型" className="h-8 px-2.5 text-fine rounded-[var(--radius-md)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-28 outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" onChange={(e) => updateParam('action', e.target.value)} /></div>
        <div><label htmlFor={idType} className="block text-fine text-[var(--color-text-muted-48)] mb-0.5 pl-2">对象</label>
          <input id={idType} placeholder="类型" className="h-8 px-2.5 text-fine rounded-[var(--radius-md)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-24 outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" onChange={(e) => updateParam('target_type', e.target.value)} /></div>
        <div><label htmlFor={idFrom} className="block text-fine text-[var(--color-text-muted-48)] mb-0.5 pl-2">开始</label>
          <input id={idFrom} type="date" className="h-8 px-2.5 text-fine rounded-[var(--radius-md)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-32 outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" onChange={(e) => updateParam('date_from', e.target.value)} /></div>
        <div><label htmlFor={idTo} className="block text-fine text-[var(--color-text-muted-48)] mb-0.5 pl-2">结束</label>
          <input id={idTo} type="date" className="h-8 px-2.5 text-fine rounded-[var(--radius-md)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-32 outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" onChange={(e) => updateParam('date_to', e.target.value)} /></div>
      </div>
      {error && <p className="text-[var(--color-error)] text-caption mb-4">加载失败，请刷新重试</p>}
      {!error && data?.items?.length === 0 ? (
        <EmptyState icon={<ScrollText size={40} />} title="暂无审计日志" description="系统操作记录将显示在这里" />
      ) : (
        <>
          <AppleTable
            columns={[
              { key: '_check', title: <BatchSelectHeader items={items} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} onSelectAll={batch.selectAll} />, render: (r) => <BatchSelectRow row={r} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} />, width: '44px' },
              { key: 'operator_name', title: '操作人' },
              { key: 'action', title: '操作', render: (r) => <span className="text-caption">{r.action}</span> },
              { key: 'target_type', title: '对象类型' },
              { key: 'ip_address', title: 'IP' },
              { key: 'created_at', title: '时间', render: (r) => formatDate(r.created_at) },
            ]}
            data={items} loading={!data && !error} rowKey="id"
          />
          {data && <ApplePagination page={Number(params.page)} pageSize={10} total={data.total} onChange={changePage} />}
        </>
      )}
      <ConfirmDialog open={batch.confirmDelete} onOpenChange={batch.setConfirmDelete}
        title="批量删除审计日志"
        message={`确定要删除 ${batch.selectedIds.size} 条审计日志吗？此操作不可撤销。`}
        onConfirm={async () => { await batch.handleBatchDelete(); toast.success('已删除'); }} loading={batch.deleting} danger confirmLabel="删除" />
    </div>
  );
}
