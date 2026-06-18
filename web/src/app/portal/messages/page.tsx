'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { getMessages, markAsRead } from '@/lib/api/message';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { formatDate } from '@/lib/date';
import { useRouter } from 'next/navigation';
import { useToast } from '@/hooks/useToast';
import styles from './page.module.css';

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

  return (
    <div>
      <h1 className={styles.title}>站内消息</h1>
      {error && <p className={styles.error}>加载失败</p>}
      <AppleTable
        columns={[
          { key: 'title', title: '标题', render: (r) => <span className={r.is_read ? '' : styles.unreadTitle}>{r.title}</span> },
          { key: 'content', title: '内容' },
          { key: 'created_at', title: '时间', render: (r) => formatDate(r.created_at) },
          { key: 'actions', title: '', render: (r) => !r.is_read ? <button onClick={() => handleRead(r.id, r.related_type, r.related_id)} className={styles.readBtn}>查看</button> : <span className={styles.readLabel}>已读</span> },
        ]}
        data={data?.items || []}
        loading={!data && !error}
        rowKey="id"
      />
      {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
    </div>
  );
}
