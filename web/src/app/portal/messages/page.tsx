'use client';
import useSWR, { mutate as globalMutate } from 'swr';
import { useState } from 'react';
import { getMessages, markAsRead, markAllRead } from '@/lib/api/message';
import { PAGE_SIZE } from '@/lib/api/constants';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { Button } from '@/components/ui/button';
import { formatDate } from '@/lib/date';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { PageTitle } from '@/components/shared/PageTitle';
import { CheckCheck, Mail, ExternalLink, Eye } from 'lucide-react';

const TYPE_LABEL: Record<string, string> = {
  ticket_supplement: '补充信息',
  ticket_resolved: '已解决',
  ticket_closed: '已关闭',
  knowledge_approved: '审核通过',
  knowledge_rejected: '审核驳回',
  knowledge_article: '知识文章',
};

/** 有有效跳转目标的消息类型 */
const NAVIGABLE_TYPES = new Set(['ticket']);

export default function MessagesPage() {
  const [page, setPage] = useState(1);
  const router = useRouter();
  const { data, error, mutate } = useSWR(`messages-${page}`, () => getMessages(page));

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

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <PageTitle>站内消息</PageTitle>
        {!isEmpty && (
          <Button variant="secondary" size="sm" onClick={handleMarkAll} disabled={!hasUnread}>
            <CheckCheck size={16} />全部已读
          </Button>
        )}
      </div>

      {error && <p className="text-[var(--color-error)] text-caption mb-4">加载失败，请刷新重试</p>}

      {isEmpty ? (
        <div className="text-center py-16">
          <Mail size={32} className="mx-auto mb-4 text-[var(--color-text-muted-48)]" />
          <p className="text-title text-[var(--color-text-muted-48)]">暂无消息</p>
        </div>
      ) : (
        <>
          <DataTable
            columns={[
              { accessorKey: 'type', header: '类型', cell: ({ row }) => <span className="text-fine text-[var(--color-text-muted-48)]">{TYPE_LABEL[row.original.type] ?? row.original.type}</span> },
              { accessorKey: 'title', header: '标题', cell: ({ row }) => <span className={row.original.is_read ? 'text-[var(--color-text-muted-80)]' : 'font-semibold'}>{row.original.title}</span> },
              { accessorKey: 'content', header: '内容', cell: ({ row }) => <span className={row.original.is_read ? 'text-[var(--color-text-muted-48)]' : ''}>{row.original.content}</span> },
              { accessorKey: 'created_at', header: '时间', cell: ({ row }) => <span className={row.original.is_read ? 'text-[var(--color-text-muted-48)]' : ''}>{formatDate(row.original.created_at)}</span> },
              { id: 'actions', header: '', cell: ({ row }) =>
                !row.original.is_read ? (
                  <Button variant="ghost" size="icon" aria-label="查看" onClick={() => handleRead(row.original.id, row.original.related_type, row.original.related_id)}><Eye /></Button>
                ) : NAVIGABLE_TYPES.has(row.original.related_type) ? (
                  <Button variant="ghost" size="icon" aria-label="跳转" onClick={() => {
                    if (row.original.related_type === 'ticket') router.push(`/portal/tickets/${row.original.related_id}`);
                  }}>
                    <ExternalLink />
                  </Button>
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
