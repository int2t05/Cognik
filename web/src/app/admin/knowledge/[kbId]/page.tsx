'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import { getArticleList } from '@/lib/api/knowledge';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { Button } from '@/components/ui/button';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { FilePlus, FileText, ChevronLeft } from 'lucide-react';
import { PageTitle } from '@/components/shared/PageTitle';
import { ListSearchInput } from '@/components/shared/ListSearchInput';
import { TableFilterHeader, type TableFilterOption } from '@/components/shared/TableFilterHeader';
import { InlineError } from '@/components/shared/InlineError';
import { EmptyState } from '@/components/shared/EmptyState';

const ARTICLE_STATUS_OPTIONS: TableFilterOption<string>[] = [
  { value: '-1', label: '全部' },
  { value: '1', label: '草稿' },
  { value: '2', label: '待审核' },
  { value: '4', label: '已发布' },
  { value: '0', label: '已停用' },
];

const ARTICLE_SOURCE_OPTIONS: TableFilterOption<number>[] = [
  { value: 0, label: '全部' },
  { value: 1, label: '手动创建' },
  { value: 2, label: '文档上传' },
];

const ARTICLE_PROCESS_OPTIONS: TableFilterOption<string>[] = [
  { value: '', label: '全部' },
  { value: 'pending', label: '等待中' },
  { value: 'completed', label: '已完成' },
  { value: 'failed', label: '失败' },
];

export default function ArticleListPage() {
  const { kbId } = useParams<{ kbId: string }>();
  const router = useRouter();
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('-1');
  const [sourceType, setSourceType] = useState(0);
  const [processStatus, setProcessStatus] = useState('');
  const [keyword, setKeyword] = useState('');
  const { data, error } = useSWR(`articles-${kbId}-${page}-${status}-${sourceType}-${processStatus}-${keyword}`, () => getArticleList(Number(kbId), page, status, keyword, sourceType, processStatus), { keepPreviousData: true });

  const isEmpty = !error && data && (data.items || []).length === 0;
  const hasFilters = status !== '-1' || sourceType !== 0 || processStatus !== '' || keyword !== '';
  const clearFilters = () => { setStatus('-1'); setSourceType(0); setProcessStatus(''); setKeyword(''); setPage(1); };

  return (
    <div>
      <div className="flex justify-between items-center mb-5">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => router.push('/admin/knowledge')} aria-label="返回"><ChevronLeft /></Button>
          <PageTitle className="mb-0">知识文章</PageTitle>
        </div>
        <Button size="icon" onClick={() => router.push(`/admin/knowledge/${kbId}/new`)} aria-label="新建文章"><FilePlus /></Button>
      </div>
      {error && <InlineError />}
      <div className="mb-4">
        <ListSearchInput value={keyword} onDebouncedChange={(v) => { setKeyword(v); setPage(1); }} placeholder="搜索标题、标签…" />
      </div>
      {isEmpty ? (
        <EmptyState
          icon={<FileText size={40} />}
          title={hasFilters ? '未找到匹配的文章' : '暂无文章'}
          description={hasFilters ? '尝试调整筛选条件或清除筛选' : '点击右上角新建文章'}
          onClearFilters={hasFilters ? clearFilters : undefined}
        />
      ) : (
        <>
          <DataTable
            columns={[
              { accessorKey: 'title', header: '标题', cell: ({ row }) => <Link href={`/admin/knowledge/${kbId}/${row.original.id}`} className="text-[var(--color-accent)]">{row.original.title}</Link> },
              { id: 'source_type_text', meta: { width: '80px' }, header: () => <TableFilterHeader label="来源" value={sourceType} options={ARTICLE_SOURCE_OPTIONS} onChange={(v) => { setSourceType(v); setPage(1); }} />, cell: ({ row }) => <span className="text-fine">{row.original.source_type === 1 ? '手动' : '上传'}</span> },
              { accessorKey: 'status', meta: { width: '100px' }, header: () => <TableFilterHeader label="状态" value={status} options={ARTICLE_STATUS_OPTIONS} onChange={(v) => { setStatus(v); setPage(1); }} />, cell: ({ row }) => <StatusBadge type="article" status={row.original.status} /> },
              { accessorKey: 'process_status', meta: { width: '110px' }, header: () => <TableFilterHeader label="处理" value={processStatus} options={ARTICLE_PROCESS_OPTIONS} onChange={(v) => { setProcessStatus(v); setPage(1); }} />, cell: ({ row }) => row.original.process_status ? <StatusBadge type="process" status={row.original.process_status} /> : '—' },
              { id: 'created_at', meta: { width: '120px' }, header: '更新时间', cell: ({ row }) => formatDate(row.original.updated_at) },
            ]}
            data={data?.items || []}
            loading={!data && !error}
          />
          {data && <DataTablePagination page={page} pageSize={10} total={data.total} onChange={(p) => setPage(p)} />}
        </>
      )}
    </div>
  );
}
