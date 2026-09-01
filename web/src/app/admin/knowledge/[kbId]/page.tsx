'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { getArticleList } from '@/lib/api/knowledge';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { Button } from '@/components/ui/button';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { FilePlus, ListFilter, FileText, Clock, CheckCircle, XCircle, ChevronLeft, Search } from 'lucide-react';
import { PageTitle } from '@/components/shared/PageTitle';
import { FilterBar, type FilterOption } from '@/components/shared/FilterBar';

const ARTICLE_FILTERS: FilterOption<string>[] = [
  { value: '-1', label: '全部', icon: <ListFilter size={16} /> },
  { value: '1', label: '草稿', icon: <FileText size={16} /> },
  { value: '2', label: '待审核', icon: <Clock size={16} /> },
  { value: '4', label: '已发布', icon: <CheckCircle size={16} /> },
  { value: '0', label: '已停用', icon: <XCircle size={16} /> },
];

export default function ArticleListPage() {
  const { kbId } = useParams<{ kbId: string }>();
  const router = useRouter();
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('-1');
  const [keyword, setKeyword] = useState('');
  const { data, error } = useSWR(`articles-${kbId}-${page}-${status}-${keyword}`, () => getArticleList(Number(kbId), page, status, keyword));

  return (
    <div>
      <div className="flex justify-between items-center mb-5">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => router.push('/admin/knowledge')} aria-label="返回"><ChevronLeft /></Button>
          <PageTitle>知识文章</PageTitle>
        </div>
        <Button size="icon" onClick={() => router.push(`/admin/knowledge/${kbId}/new`)} aria-label="新建文章"><FilePlus /></Button>
      </div>
      {error && <p className="text-[var(--color-error)] text-caption mb-4">加载失败，请刷新重试</p>}
      <div className="flex items-center gap-2 mb-4 flex-wrap">
        <FilterBar options={ARTICLE_FILTERS} value={status} onChange={(v) => { setStatus(v); setPage(1); }} className="!mb-0" />
        <div className="relative">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--color-text-muted-48)] pointer-events-none" />
          <input
            type="text"
            placeholder="搜索标题或标签…"
            value={keyword}
            onChange={(e) => { setKeyword(e.target.value); setPage(1); }}
            className="w-56 py-2 pl-8 pr-3.5 text-caption rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none transition placeholder:text-[var(--color-text-muted-48)] focus:border-[var(--color-accent)] focus:shadow-[var(--focus-ring)]"
          />
        </div>
      </div>
      <DataTable
        columns={[
          { accessorKey: 'title', header: '标题', cell: ({ row }) => <a href={`/admin/knowledge/${kbId}/${row.original.id}`} className="text-[var(--color-accent)]">{row.original.title}</a> },
          { id: 'source_type_text', header: '来源', cell: ({ row }) => <span className="text-fine">{row.original.source_type === 1 ? '手动' : '上传'}</span> },
          { accessorKey: 'status', header: '状态', cell: ({ row }) => <StatusBadge type="article" status={row.original.status} /> },
          { accessorKey: 'process_status', header: '处理', cell: ({ row }) => row.original.process_status ? <StatusBadge type="process" status={row.original.process_status} /> : '—' },
          { id: 'created_at', header: '更新时间', cell: ({ row }) => formatDate(row.original.updated_at) },
        ]}
        data={data?.items || []}
        loading={!data && !error}
      />
      {data && <DataTablePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
    </div>
  );
}
