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
import { FilePlus, ListFilter, FileText, Clock, CheckCircle, XCircle, ArrowLeft } from 'lucide-react';

export default function ArticleListPage() {
  const { kbId } = useParams<{ kbId: string }>();
  const router = useRouter();
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('-1');
  const { data, error } = useSWR(`articles-${kbId}-${page}-${status}`, () => getArticleList(Number(kbId), page, status));

  const filterOptions = [
    { v: '-1', label: '全部', icon: <ListFilter size={17} /> },
    { v: '1', label: '草稿', icon: <FileText size={17} /> },
    { v: '2', label: '待审核', icon: <Clock size={17} /> },
    { v: '4', label: '已发布', icon: <CheckCircle size={17} /> },
    { v: '0', label: '已停用', icon: <XCircle size={17} /> },
  ];

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <div className="flex items-center gap-3">
          <AppleButton variant="ghost" onClick={() => router.push('/admin/knowledge')} className="p-1.5" aria-label="返回"><ArrowLeft size={15} /></AppleButton>
          <h1 className="text-hero font-semibold text-[var(--color-ink)]">知识文章</h1>
        </div>
        <AppleButton onClick={() => router.push(`/admin/knowledge/${kbId}/new`)} className="p-2" aria-label="新建文章"><FilePlus size={16} /></AppleButton>
      </div>
      {error && <p className="text-[var(--color-error)] text-caption mb-4">加载失败，请刷新重试</p>}
      <div className="mb-4 flex gap-2">
        {filterOptions.map((o) => (
          <button key={o.v} onClick={() => { setStatus(o.v); setPage(1); }} aria-label={o.label}
            className={`p-2 border rounded-[var(--radius-pill)] cursor-pointer transition ${status === o.v ? 'bg-[var(--color-accent)] border-[var(--color-accent)] text-[var(--color-on-accent)]' : 'bg-[var(--color-pearl)] border-[var(--color-divider-soft)] text-[var(--color-text-muted-80)] hover:border-[var(--color-hairline)]'}`}>
            {o.icon}
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
