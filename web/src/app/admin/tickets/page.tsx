'use client';
import useSWR from 'swr';
import { useState } from 'react';
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
import { TableFilterHeader, type TableFilterOption } from '@/components/shared/TableFilterHeader';
import { InlineError } from '@/components/shared/InlineError';
import { EmptyState } from '@/components/shared/EmptyState';
import { Button } from '@/components/ui/button';

const TICKET_STATUS_OPTIONS: TableFilterOption<number>[] = [
  { value: -1, label: '全部' },
  { value: 1, label: '待处理' },
  { value: 2, label: '处理中' },
  { value: 3, label: '需补充' },
  { value: 4, label: '已解决' },
  { value: 5, label: '已关闭' },
];

export default function AdminTicketListPage() {
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
      toast.success(`已关闭 ${ok} 条` + (fail ? `，${fail} 条失败（已关闭/已解决不可关闭）` : ''));
      setBatchCloseConfirm(false);
      batch.clearSelection();
      mutate();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : '批量关闭失败');
    } finally {
      setBatchClosing(false);
    }
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-5">
        <PageTitle>申告管理</PageTitle>
      </div>
      {error && <InlineError />}
      <div className="mb-4 flex items-center justify-between gap-2 flex-wrap">
        <ListSearchInput value={keyword} onDebouncedChange={(v) => { setKeyword(v); setPage(1); }} placeholder="搜索标题、编号、描述…" />
        <BatchSelectToolbar selectedCount={batch.selectedIds.size} onDelete={() => batch.setConfirmDelete(true)} onCancel={batch.clearSelection} />
        {batch.selectedIds.size > 0 && <Button size="sm" variant="ghost" onClick={() => setBatchCloseConfirm(true)}><XCircle size={14} />批量关闭</Button>}
      </div>
      {isEmpty ? (
        <EmptyState
          icon={<FileText size={40} />}
          title={hasFilters ? '未找到匹配的申告' : '暂无申告'}
          description={hasFilters ? '尝试调整筛选条件或清除筛选' : '系统中暂无申告记录'}
          onClearFilters={hasFilters ? clearFilters : undefined}
        />
      ) : (
        <>
          <DataTable
            columns={[
              { id: '_check', meta: { width: '40px' }, header: () => <BatchSelectHeader items={items} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} onSelectAll={batch.selectAll} />, cell: ({ row }) => <BatchSelectRow row={row.original} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} /> },
              { accessorKey: 'ticket_no', meta: { width: '120px' }, header: '编号', cell: ({ row }) => <span className="font-[var(--font-mono)] text-fine">{row.original.ticket_no}</span> },
              { accessorKey: 'title', header: '标题', cell: ({ row }) => <Link href={`/admin/tickets/${row.original.id}`} className="text-[var(--color-accent)]">{row.original.title}</Link> },
              { accessorKey: 'submitter_name', meta: { width: '88px' }, header: '提交人' },
              { accessorKey: 'tags', meta: { width: '120px' }, header: '标签', cell: ({ row }) => (row.original.tags || []).join(', ') || '-' },
              { accessorKey: 'status', meta: { width: '88px' }, header: () => <TableFilterHeader label="状态" value={status} options={TICKET_STATUS_OPTIONS} onChange={(v) => { setStatus(v); setPage(1); }} />, cell: ({ row }) => <StatusBadge type="ticket" status={row.original.status} /> },
              { accessorKey: 'created_at', meta: { width: '120px' }, header: '提交时间', cell: ({ row }) => formatDate(row.original.created_at) },
            ]}
            data={items} loading={!data && !error}
          />
          {data && <DataTablePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
        </>
      )}
      <ConfirmDialog open={batch.confirmDelete} onOpenChange={batch.setConfirmDelete}
        title="批量删除申告"
        message={`确定要删除 ${batch.selectedIds.size} 条申告吗？此操作不可撤销。`}
        onConfirm={async () => { await batch.handleBatchDelete(); toast.success('已删除'); }} loading={batch.deleting} danger confirmLabel="删除" />
      <ConfirmDialog open={batchCloseConfirm} onOpenChange={setBatchCloseConfirm}
        title="批量关闭申告"
        message={`确定要关闭 ${batch.selectedIds.size} 条申告吗？已解决/已关闭的申告将被跳过。`}
        onConfirm={handleBatchClose} loading={batchClosing} confirmLabel="关闭" />
    </div>
  );
}
