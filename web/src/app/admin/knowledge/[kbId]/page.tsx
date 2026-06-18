'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { getArticleList, deleteKB } from '@/lib/api/knowledge';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { AppleButton } from '@/components/ui/AppleButton';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { useToast } from '@/hooks/useToast';
import styles from './page.module.css';

export default function ArticleListPage() {
  const { kbId } = useParams<{ kbId: string }>();
  const router = useRouter();
  const toast = useToast();
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('-1');
  const { data, error } = useSWR(`articles-${kbId}-${page}-${status}`, () => getArticleList(Number(kbId), page, status));

  const filterOptions = [
    { v: '-1', l: '全部' },
    { v: '1', l: '草稿' },
    { v: '2', l: '待审核' },
    { v: '4', l: '已发布' },
    { v: '0', l: '已停用' },
  ];

  return (
    <div>
      <div className={styles.header}>
        <h1 className={styles.title}>知识文章</h1>
        <AppleButton onClick={() => router.push(`/admin/knowledge/${kbId}/new`)}>新建文章</AppleButton>
      </div>
      <div className={styles.filterBar}>
        {filterOptions.map((o) => (
          <button key={o.v} onClick={() => { setStatus(o.v); setPage(1); }}
            className={`${styles.filterBtn} ${status === o.v ? styles.filterBtnActive : ''}`}>
            {o.l}
          </button>
        ))}
      </div>
      <AppleTable
        columns={[
          { key: 'title', title: '标题', render: (r) => <a href={`/admin/knowledge/${kbId}/${r.id}`} className={styles.link}>{r.title}</a> },
          { key: 'source_type_text', title: '来源', render: (r) => <span className={styles.mono}>{r.source_type === 1 ? '手动' : '上传'}</span> },
          { key: 'status', title: '状态', render: (r) => <StatusBadge type="article" status={r.status} /> },
          { key: 'process_status', title: '处理', render: (r) => r.process_status ? <StatusBadge type="process" status={r.process_status} /> : '—' },
          { key: 'created_at', title: '更新时间', render: (r) => formatDate(r.updated_at) },
        ]}
        data={data?.items || []}
        loading={!data && !error}
        rowKey="id"
      />
      {data && <ApplePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
    </div>
  );
}
