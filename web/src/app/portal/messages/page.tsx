'use client';
import useSWR, { mutate as globalMutate } from 'swr';
import { useState } from 'react';
import { getMessages, markAsRead, markAllRead } from '@/lib/api/message';
import { PAGE_SIZE } from '@/lib/api/constants';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { IconButton } from '@/components/ui/icon-button';
import { formatDate } from '@/lib/date';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { PageTitle } from '@/components/shared/PageTitle';
import { InlineError } from '@/components/shared/InlineError';
import { EmptyState } from '@/components/shared/EmptyState';
import { TableFilterHeader, type TableFilterOption } from '@/components/shared/TableFilterHeader';
import { CheckCheck, Mail, ExternalLink, Eye } from 'lucide-react';

const TYPE_LABEL: Record<string, string> = {
  ticket_supplement: '补充信息',
  ticket_resolved: '已解决',
  ticket_closed: '已关闭',
  ticket_overdue: '处理超时',
  knowledge_approved: '审核通过',
  knowledge_rejected: '审核驳回',
  knowledge_article: '知识文章',
};

const MESSAGE_TYPE_OPTIONS: TableFilterOption<string>[] = [
  { value: '', label: '全部' },
  { value: 'ticket_supplement', label: '补充信息' },
  { value: 'ticket_resolved', label: '已解决' },
  { value: 'ticket_closed', label: '已关闭' },
  { value: 'ticket_overdue', label: '处理超时' },
  { value: 'knowledge_approved', label: '审核通过' },
  { value: 'knowledge_rejected', label: '审核驳回' },
];

/** 有有效跳转目标的消息类型 */
const NAVIGABLE_TYPES = new Set(['ticket']);

export default function MessagesPage() {
  const [page, setPage] = useState(1);
  const [type, setType] = useState('');
  const router = useRouter();
  const { data, error, mutate } = useSWR(`messages-${page}-${type}`, () => getMessages(page, type), { keepPreviousData: true });

  const handleRead = async (id: number, relatedType: string, relatedId: number) => {
    try {
      await markAsRead(id);
      mutate();
      globalMutate('unread-count');
      if (relatedType === 'ticket') router.push(`/portal/tickets/${relatedId}`);
    } catch {
      toast.error('标记已读失败');
    }
  };

  const handleMarkAll = async () => {
    try {
      const res = await markAllRead();
      toast.success(res.affected > 0 ? `已标记 ${res.affected} 条消息为已读` : '没有未读消息');
      mutate();
      globalMutate('unread-count');
    } catch {
      toast.error('操作失败');
    }
  };

  const messages = data?.items ?? [];
  const hasUnread = messages.some((m) => !m.is_read);
  const isEmpty = !error && data && messages.length === 0;
  const hasFilters = type !== '';
  const clearFilters = () => { setType(''); setPage(1); };

  return (
    <div className="min-w-0 overflow-hidden">
      <div className="flex items-center justify-between mb-5">
        <PageTitle className="mb-0">站内消息</PageTitle>
        {!isEmpty && (
          <IconButton variant="secondary" size="sm" onClick={handleMarkAll} disabled={!hasUnread}>
            <CheckCheck size={16} />全部已读
          </IconButton>
        )}
      </div>

      {error && <InlineError />}

      {isEmpty ? (
        <EmptyState icon={<Mail size={40} />} title={hasFilters ? '未找到匹配的消息' : '暂无消息'} onClearFilters={hasFilters ? clearFilters : undefined} />
      ) : (
        <>
          <DataTable
            columns={[
              { accessorKey: 'type', meta: { width: '88px' }, header: () => <TableFilterHeader label="类型" value={type} options={MESSAGE_TYPE_OPTIONS} onChange={(v) => { setType(v); setPage(1); }} />, cell: ({ row }) => <span className="text-fine text-[var(--color-text-muted-48)]">{TYPE_LABEL[row.original.type] ?? row.original.type}</span> },
              { accessorKey: 'title', meta: { width: '120px' }, header: '标题', cell: ({ row }) => <span className={row.original.is_read ? 'text-[var(--color-text-muted-80)]' : 'font-semibold'}>{row.original.title}</span> },
              { accessorKey: 'content', header: '内容', cell: ({ row }) => <span className={row.original.is_read ? 'text-[var(--color-text-muted-48)]' : ''}>{row.original.content}</span> },
              { accessorKey: 'created_at', meta: { width: '120px' }, header: '时间', cell: ({ row }) => <span className={row.original.is_read ? 'text-[var(--color-text-muted-48)]' : ''}>{formatDate(row.original.created_at)}</span> },
              { id: 'actions', meta: { width: '60px' }, header: '', cell: ({ row }) =>
                !row.original.is_read ? (
                  <IconButton label="查看" onClick={() => handleRead(row.original.id, row.original.related_type, row.original.related_id)}><Eye /></IconButton>
                ) : NAVIGABLE_TYPES.has(row.original.related_type) ? (
                  <IconButton label="跳转" onClick={() => {
                    if (row.original.related_type === 'ticket') router.push(`/portal/tickets/${row.original.related_id}`);
                  }}>
                    <ExternalLink />
                  </IconButton>
                ) : null
              },
            ]}
            data={messages}
            loading={!data && !error}
          />
          {data && <DataTablePagination page={page} pageSize={PAGE_SIZE} total={data.total} onChange={(p) => setPage(p)} />}
        </>
      )}
    </div>
  );
}
