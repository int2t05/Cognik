'use client';
import useSWR from 'swr';
import { useState } from 'react';
import Link from 'next/link';
import { listAllTickets, batchDeleteTickets } from '@/lib/api/ticket';
import { useBatchSelection } from '@/hooks/useBatchSelection';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { Input } from '@/components/ui/input';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { BatchSelectHeader, BatchSelectRow, BatchSelectToolbar } from '@/components/chat/BatchSelectCheckbox';
import { formatDate } from '@/lib/date';
import { ListFilter, Clock, AlertCircle, CheckCircle, XCircle, MessageSquare, Search, X } from 'lucide-react';
import { useDebounce } from '@/hooks/useDebounce';
import { toast } from 'sonner';
import { PageTitle } from '@/components/shared/PageTitle';
import { FilterBar, type FilterOption } from '@/components/shared/FilterBar';

const TICKET_FILTERS: FilterOption<number>[] = [
  { value: -1, label: '全部', icon: <ListFilter size={16} /> },
  { value: 1, label: '待处理', icon: <AlertCircle size={16} /> },
  { value: 2, label: '处理中', icon: <Clock size={16} /> },
  { value: 3, label: '需补充', icon: <MessageSquare size={16} /> },
  { value: 4, label: '已解决', icon: <CheckCircle size={16} /> },
  { value: 5, label: '已关闭', icon: <XCircle size={16} /> },
];

export default function AdminTicketListPage() {
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState(-1);
  const [keyword, setKeyword] = useState('');
  const debouncedKeyword = useDebounce(keyword, 300);
  const { data, error, mutate } = useSWR(`admin-tickets-${page}-${status}`, () => listAllTickets(page, status));

  const items = (data?.items || []).filter((t: { title?: string; ticket_no?: string; submitter_name?: string }) => {
    if (!debouncedKeyword) return true;
    const kw = debouncedKeyword.toLowerCase();
    return (t.title?.toLowerCase().includes(kw)) || (t.ticket_no?.toLowerCase().includes(kw)) || (t.submitter_name?.toLowerCase().includes(kw));
  });

  const batch = useBatchSelection({
    items,
    batchDeleteFn: batchDeleteTickets,
    onMutate: () => mutate(),
    onError: (msg) => toast.error(msg),
  });

  return (
    <div>
      <div className="flex justify-between items-center mb-5">
        <PageTitle>申告管理</PageTitle>
      </div>
      {error && <p className="text-[var(--color-error)] text-caption mb-4">加载失败，请刷新重试</p>}
      <div className="mb-4 flex gap-2 items-center flex-wrap">
        <div className="relative">
          <Search size={14} className="absolute left-4 top-1/2 -translate-y-1/2 text-[var(--color-text-muted-48)] pointer-events-none" />
          <Input placeholder="搜索编号/标题/提交人..." aria-label="搜索申告" value={keyword} onChange={(e) => { setKeyword(e.target.value); setPage(1); }} className="h-11 rounded-[var(--radius-pill)] pl-10 pr-10 min-w-[100px]" />
          {keyword && (
            <Button type="button" variant="ghost" size="icon" onClick={() => { setKeyword(''); setPage(1); }} aria-label="清除搜索" className="absolute right-1 top-1/2 -translate-y-1/2 size-7 text-[var(--color-text-muted-48)] hover:text-[var(--color-ink)]">
              <X size={12} />
            </Button>
          )}
        </div>
        <FilterBar options={TICKET_FILTERS} value={status} onChange={(v) => { setStatus(v); setPage(1); }} />
        <BatchSelectToolbar selectedCount={batch.selectedIds.size} onDelete={() => batch.setConfirmDelete(true)} onCancel={batch.clearSelection} />
      </div>
      <DataTable
        columns={[
          { id: '_check', header: () => <BatchSelectHeader items={items} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} onSelectAll={batch.selectAll} />, cell: ({ row }) => <BatchSelectRow row={row.original} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} /> },
          { accessorKey: 'ticket_no', header: '编号', cell: ({ row }) => <span className="font-[var(--font-mono)] text-fine">{row.original.ticket_no}</span> },
          { accessorKey: 'title', header: '标题', cell: ({ row }) => <Link href={`/admin/tickets/${row.original.id}`} className="text-[var(--color-accent)]">{row.original.title}</Link> },
          { accessorKey: 'submitter_name', header: '提交人' },
          { accessorKey: 'tags', header: '标签', cell: ({ row }) => (row.original.tags || []).join(', ') || '-' },
          { accessorKey: 'status', header: '状态', cell: ({ row }) => <StatusBadge type="ticket" status={row.original.status} /> },
          { accessorKey: 'created_at', header: '提交时间', cell: ({ row }) => formatDate(row.original.created_at) },
        ]}
        data={items} loading={!data && !error}
      />
      {data && <DataTablePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
      <ConfirmDialog open={batch.confirmDelete} onOpenChange={batch.setConfirmDelete}
        title="批量删除申告"
        message={`确定要删除 ${batch.selectedIds.size} 条申告吗？此操作不可撤销。`}
        onConfirm={async () => { await batch.handleBatchDelete(); toast.success('已删除'); }} loading={batch.deleting} danger confirmLabel="删除" />
    </div>
  );
}
