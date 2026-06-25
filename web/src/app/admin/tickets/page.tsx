'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { listAllTickets, batchDeleteTickets } from '@/lib/api/ticket';
import { useBatchSelection } from '@/hooks/useBatchSelection';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { SearchInput } from '@/components/ui/SearchInput';
import { AppleButton } from '@/components/ui/AppleButton';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { BatchSelectHeader, BatchSelectRow, BatchSelectToolbar } from '@/components/chat/BatchSelectCheckbox';
import { formatDate } from '@/lib/date';
import { ListFilter, Clock, AlertCircle, CheckCircle, XCircle, MessageSquare } from 'lucide-react';
import { useDebounce } from '@/hooks/useDebounce';
import { useToast } from '@/hooks/useToast';
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
  const toast = useToast();

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
        <SearchInput placeholder="搜索编号/标题/提交人..." aria-label="搜索申告" value={keyword} onChange={(e) => { setKeyword(e.target.value); setPage(1); }} className="min-w-[100px]" />
        <FilterBar options={TICKET_FILTERS} value={status} onChange={(v) => { setStatus(v); setPage(1); }} />
        <BatchSelectToolbar selectedCount={batch.selectedIds.size} onDelete={() => batch.setConfirmDelete(true)} onCancel={batch.clearSelection} />
      </div>
      <AppleTable
        columns={[
          { key: '_check', title: <BatchSelectHeader items={items} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} onSelectAll={batch.selectAll} />, render: (r) => <BatchSelectRow row={r} selectedIds={batch.selectedIds} onToggleSelect={batch.toggleSelect} />, width: '44px' },
          { key: 'ticket_no', title: '编号', render: (r) => <span className="font-[var(--font-mono)] text-fine">{r.ticket_no}</span> },
          { key: 'title', title: '标题', render: (r) => <a href={`/admin/tickets/${r.id}`} className="text-[var(--color-accent)]">{r.title}</a> },
          { key: 'submitter_name', title: '提交人' },
          { key: 'tags', title: '标签', render: (r) => (r.tags || []).join(', ') || '-' },
          { key: 'status', title: '状态', render: (r) => <StatusBadge type="ticket" status={r.status} /> },
          { key: 'created_at', title: '提交时间', render: (r) => formatDate(r.created_at) },
        ]}
        data={items} loading={!data && !error} rowKey="id"
      />
      {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
      <ConfirmDialog open={batch.confirmDelete} onOpenChange={batch.setConfirmDelete}
        title="批量删除申告"
        message={`确定要删除 ${batch.selectedIds.size} 条申告吗？此操作不可撤销。`}
        onConfirm={async () => { await batch.handleBatchDelete(); toast.success('已删除'); }} loading={batch.deleting} danger confirmLabel="删除" />
    </div>
  );
}
