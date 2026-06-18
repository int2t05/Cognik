'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { getMessages, markAsRead } from '@/lib/api/message';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { formatDate } from '@/lib/date';
import { useRouter } from 'next/navigation';

export default function MessagesPage() {
  const [page, setPage] = useState(1);
  const router = useRouter();
  const { data, error, mutate } = useSWR(`messages-${page}`, () => getMessages(page));

  const handleRead = async (id: number, relatedType: string, relatedId: number) => {
    await markAsRead(id);
    mutate();
    if (relatedType === 'ticket') router.push(`/portal/tickets/${relatedId}`);
  };

  return (
    <div>
      <h1 style={{ fontSize: 28, fontWeight: 600, color: 'var(--text-ink)', marginBottom: 24 }}>站内消息</h1>
      {error && <p style={{ color: 'var(--color-error)' }}>加载失败</p>}
      <AppleTable
        columns={[
          { key: 'title', title: '标题', render: (r) => <span style={{ fontWeight: r.is_read ? 400 : 600 }}>{r.title}</span> },
          { key: 'content', title: '内容' },
          { key: 'created_at', title: '时间', render: (r) => formatDate(r.created_at) },
          { key: 'actions', title: '', render: (r) => !r.is_read ? <button onClick={() => handleRead(r.id, r.related_type, r.related_id)} style={{ border: 'none', background: 'none', color: 'var(--accent)', cursor: 'pointer', fontSize: 14 }}>查看</button> : <span style={{ color: 'var(--text-muted-48)', fontSize: 13 }}>已读</span> },
        ]}
        data={data?.items || []}
        loading={!data && !error}
        rowKey="id"
      />
      {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
    </div>
  );
}
