'use client';
import useSWR from 'swr';
import { useState, useId } from 'react';
import { getAuditLogs, batchDeleteAuditLogs } from '@/lib/api/audit';
import { PageTitle } from '@/components/shared/PageTitle';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { AppleButton } from '@/components/ui/AppleButton';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { formatDate } from '@/lib/date';
import { useDebounce } from '@/hooks/useDebounce';
import { useToast } from '@/hooks/useToast';
import { EmptyState } from '@/components/shared/EmptyState';
import { ScrollText, Trash2, X } from 'lucide-react';

export default function AuditLogPage() {
  const [params, setParams] = useState<Record<string, string | number>>({ page: 1, page_size: 10 });
  const debouncedParams = useDebounce(params, 300);
  const { data, error, mutate } = useSWR(`audit-${JSON.stringify(debouncedParams)}`, () => getAuditLogs(debouncedParams));
  const toast = useToast();
  const idOp = useId(); const idAct = useId(); const idType = useId(); const idFrom = useId(); const idTo = useId();

  // 批量选择
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const items = data?.items || [];
  const toggleSelect = (id: number) => setSelectedIds((prev) => {
    const next = new Set(prev);
    next.has(id) ? next.delete(id) : next.add(id);
    return next;
  });
  const selectAll = () => {
    if (selectedIds.size === items.length && items.length > 0) setSelectedIds(new Set());
    else setSelectedIds(new Set(items.map((t) => t.id)));
  };
  const clearSelection = () => setSelectedIds(new Set());
  const handleBatchDelete = async () => {
    setDeleting(true);
    try {
      await batchDeleteAuditLogs([...selectedIds]);
      toast.success(`已删除 ${selectedIds.size} 条日志`);
      clearSelection();
      mutate();
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '删除失败'); }
    finally { setDeleting(false); setConfirmDelete(false); }
  };

  const updateParam = (k: string, v: string) => setParams((prev) => ({ ...prev, [k]: v, page: 1, page_size: 10 }));
  const changePage = (p: number) => setParams((prev) => ({ ...prev, page: p }));

  return (
    <div>
      <PageTitle>审计日志</PageTitle>
      <div className="flex gap-2 mb-4 flex-wrap items-end">
        <div>
          <label htmlFor={idOp} className="block text-fine text-[var(--color-text-muted-48)] mb-0.5 pl-2">操作人</label>
          <input id={idOp} placeholder="ID" type="number" className="h-8 px-2.5 text-fine rounded-[var(--radius-md)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-24 outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" onChange={(e) => updateParam('operator_id', e.target.value)} />
        </div>
        <div>
          <label htmlFor={idAct} className="block text-fine text-[var(--color-text-muted-48)] mb-0.5 pl-2">操作</label>
          <input id={idAct} placeholder="类型" className="h-8 px-2.5 text-fine rounded-[var(--radius-md)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-28 outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" onChange={(e) => updateParam('action', e.target.value)} />
        </div>
        <div>
          <label htmlFor={idType} className="block text-fine text-[var(--color-text-muted-48)] mb-0.5 pl-2">对象</label>
          <input id={idType} placeholder="类型" className="h-8 px-2.5 text-fine rounded-[var(--radius-md)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-24 outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" onChange={(e) => updateParam('target_type', e.target.value)} />
        </div>
        <div>
          <label htmlFor={idFrom} className="block text-fine text-[var(--color-text-muted-48)] mb-0.5 pl-2">开始</label>
          <input id={idFrom} type="date" className="h-8 px-2.5 text-fine rounded-[var(--radius-md)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-32 outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" onChange={(e) => updateParam('date_from', e.target.value)} />
        </div>
        <div>
          <label htmlFor={idTo} className="block text-fine text-[var(--color-text-muted-48)] mb-0.5 pl-2">结束</label>
          <input id={idTo} type="date" className="h-8 px-2.5 text-fine rounded-[var(--radius-md)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] w-32 outline-none focus-visible:border-[var(--color-accent)] focus-visible:shadow-[var(--focus-ring)]" onChange={(e) => updateParam('date_to', e.target.value)} />
        </div>
        {selectedIds.size > 0 && (
          <span className="inline-flex items-center gap-1.5 ml-2 pl-2 border-l border-[var(--color-divider-soft)]">
            <span className="text-fine text-[var(--color-text-muted-80)]">已选 <strong>{selectedIds.size}</strong></span>
            <AppleButton variant="ghost" icon={<Trash2 />} className="text-[var(--color-error)]" onClick={() => setConfirmDelete(true)}>删除</AppleButton>
            <AppleButton variant="ghost" icon={<X />} onClick={clearSelection}>取消</AppleButton>
          </span>
        )}
      </div>
      {error && <p className="text-[var(--color-error)] text-caption mb-4">加载失败，请刷新重试</p>}
      {!error && data?.items?.length === 0 ? (
        <EmptyState icon={<ScrollText size={40} />} title="暂无审计日志" description="系统操作记录将显示在这里" />
      ) : (
        <>
          <AppleTable
            columns={[
              { key: '_check', title: (
                <input type="checkbox" checked={items.length > 0 && selectedIds.size === items.length} onChange={selectAll}
                  className="w-4 h-4 rounded border-[var(--color-hairline)] accent-[var(--color-accent)]" />
              ), render: (r) => (
                <input type="checkbox" checked={selectedIds.has(r.id)} onChange={() => toggleSelect(r.id)}
                  className="w-4 h-4 rounded border-[var(--color-hairline)] accent-[var(--color-accent)]" />
              ), width: '44px' },
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
      <ConfirmDialog open={confirmDelete} onOpenChange={setConfirmDelete}
        title="批量删除审计日志"
        message={`确定要删除 ${selectedIds.size} 条审计日志吗？此操作不可撤销。`}
        onConfirm={handleBatchDelete} loading={deleting} danger confirmLabel="删除" />
    </div>
  );
}
