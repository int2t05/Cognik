'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { getMessages, markAsRead } from '@/lib/api/message';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { AppleButton } from '@/components/ui/AppleButton';
import { formatDate } from '@/lib/date';
import { useRouter } from 'next/navigation';
import { useToast } from '@/hooks/useToast';
import { PageTitle } from '@/components/shared/PageTitle';
import { ArrowRight, Mail } from 'lucide-react';

export default function MessagesPage() {
  const [page, setPage] = useState(1);
  const router = useRouter();
  const toast = useToast();
  const { data, error, mutate } = useSWR(`messages-${page}`, () => getMessages(page));

  const handleRead = async (id: number, relatedType: string, relatedId: number) => {
    try {
      await markAsRead(id);
      mutate();
      if (relatedType === 'ticket') router.push(`/portal/tickets/${relatedId}`);
    } catch {
      toast.error('标记已读失败');
    }
  };

  const messages = data?.items ?? [];
  const isEmpty = !error && data && messages.length === 0;

  return (
    <div>
      <PageTitle>站内消息</PageTitle>
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
              { key: 'title', title: '标题', render: (r) => <span className={r.is_read ? '' : 'font-semibold'}>{r.title}</span> },
              { key: 'content', title: '内容' },
              { key: 'created_at', title: '时间', render: (r) => formatDate(r.created_at) },
              { key: 'actions', title: '操作', render: (r) => !r.is_read ? <AppleButton variant="ghost" icon={<ArrowRight />} aria-label="查看" onClick={() => handleRead(r.id, r.related_type, r.related_id)} /> : <span className="text-[var(--color-text-muted-48)] text-caption">已读</span> },
            ]}
            data={messages}
            loading={!data && !error}
            rowKey="id"
          />
          {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
        </>
      )}
    </div>
  );
}
