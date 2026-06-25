'use client';
import useSWR, { mutate as globalMutate } from 'swr';
import { useState } from 'react';
import { getMessages, markAsRead, markAllRead } from '@/lib/api/message';
import { PAGE_SIZE } from '@/lib/api/constants';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { AppleButton } from '@/components/ui/AppleButton';
import { formatDate } from '@/lib/date';
import { useRouter } from 'next/navigation';
import { useToast } from '@/hooks/useToast';
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
  const toast = useToast();
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
          <AppleButton variant="utility" icon={<CheckCheck />} onClick={handleMarkAll} disabled={!hasUnread}>
            全部已读
          </AppleButton>
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
          <AppleTable
            columns={[
              { key: 'type', title: '类型', width: '100px', render: (r) => <span className="text-fine text-[var(--color-text-muted-48)]">{TYPE_LABEL[r.type] ?? r.type}</span> },
              { key: 'title', title: '标题', render: (r) => <span className={r.is_read ? 'text-[var(--color-text-muted-80)]' : 'font-semibold'}>{r.title}</span> },
              { key: 'content', title: '内容', render: (r) => <span className={r.is_read ? 'text-[var(--color-text-muted-48)]' : ''}>{r.content}</span> },
              { key: 'created_at', title: '时间', render: (r) => <span className={r.is_read ? 'text-[var(--color-text-muted-48)]' : ''}>{formatDate(r.created_at)}</span> },
              { key: 'actions', title: '', width: '60px', render: (r) =>
                !r.is_read ? (
                  <AppleButton variant="ghost" icon={<Eye />} aria-label="查看" onClick={() => handleRead(r.id, r.related_type, r.related_id)} />
                ) : NAVIGABLE_TYPES.has(r.related_type) ? (
                  <AppleButton variant="ghost" icon={<ExternalLink />} aria-label="跳转" onClick={() => {
                    if (r.related_type === 'ticket') router.push(`/portal/tickets/${r.related_id}`);
                  }} />
                ) : null
              },
            ]}
            data={messages}
            loading={!data && !error}
            rowKey="id"
          />
          {data && <ApplePagination page={page} pageSize={PAGE_SIZE} total={data.total} onChange={(p) => setPage(p)} />}
        </>
      )}
    </div>
  );
}
