'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { getArticleList } from '@/lib/api/knowledge';
import { AppleTable } from '@/components/ui/AppleTable';
import { ApplePagination } from '@/components/ui/ApplePagination';
import { AppleButton } from '@/components/ui/AppleButton';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';

export default function ArticleListPage() {
  const { kbId } = useParams<{ kbId: string }>();
  const router = useRouter();
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
      <div className="flex items-center gap-3 mb-6">
        <AppleButton variant="ghost" onClick={() => router.push('/admin/knowledge')}>← 返回</AppleButton>
      </div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-hero font-medium text-[var(--color-ink)]">知识文章</h1>
        <AppleButton onClick={() => router.push(`/admin/knowledge/${kbId}/new`)}>新建文章</AppleButton>
      </div>
      <div className="mb-4 flex gap-2">
        {filterOptions.map((o) => (
          <button key={o.v} onClick={() => { setStatus(o.v); setPage(1); }}
            className={`px-3.5 py-1.5 border-0 rounded-[var(--radius-pill)] text-caption cursor-pointer transition hover:bg-[var(--color-hairline)] ${status === o.v ? 'bg-[var(--color-accent)] text-[var(--color-on-accent)] font-semibold' : 'bg-[var(--color-divider-soft)] text-[var(--color-ink)]'}`}>
            {o.l}
          </button>
        ))}
      </div>
      <AppleTable
        columns={[
          { key: 'title', title: '标题', render: (r) => <a href={`/admin/knowledge/${kbId}/${r.id}`} className="text-[var(--color-accent)]">{r.title}</a> },
          { key: 'source_type_text', title: '来源', render: (r) => <span className="text-fine">{r.source_type === 1 ? '手动' : '上传'}</span> },
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
