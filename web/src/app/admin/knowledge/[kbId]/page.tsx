'use client';
import useSWR from 'swr';
import { useState } from 'react';
import { useTranslations, useLocale } from 'next-intl';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import { getArticleList } from '@/lib/api/knowledge';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { IconButton } from '@/components/ui/icon-button';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { FilePlus, FileText, ChevronLeft } from 'lucide-react';
import { PageTitle } from '@/components/shared/PageTitle';
import { ListSearchInput } from '@/components/shared/ListSearchInput';
import { TableFilterHeader, type TableFilterOption } from '@/components/shared/TableFilterHeader';
import { InlineError } from '@/components/shared/InlineError';
import { EmptyState } from '@/components/shared/EmptyState';

/** 文章状态筛选 → i18n 键（复用 status.article.*） */
const ARTICLE_STATUS_KEYS: Record<string, string> = {
  '-1': 'common.all',
  '1': 'status.article.draft',
  '2': 'status.article.pending',
  '4': 'status.article.published',
  '0': 'status.article.disabled',
};

/** 文章来源筛选 → i18n 键 */
const ARTICLE_SOURCE_KEYS: Record<number, string> = {
  0: 'common.all',
  1: 'kb.sourceManual',
  2: 'kb.sourceUpload',
};

/** 文章处理状态筛选 → i18n 键（复用 status.process.*） */
const ARTICLE_PROCESS_KEYS: Record<string, string> = {
  '': 'common.all',
  pending: 'status.process.pending',
  completed: 'status.process.completed',
  failed: 'status.process.failed',
};

export default function ArticleListPage() {
  const t = useTranslations();
  const locale = useLocale();
  const { kbId } = useParams<{ kbId: string }>();
  const router = useRouter();
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState('-1');
  const [sourceType, setSourceType] = useState(0);
  const [processStatus, setProcessStatus] = useState('');
  const [keyword, setKeyword] = useState('');
  const { data, error } = useSWR(`articles-${kbId}-${page}-${status}-${sourceType}-${processStatus}-${keyword}`, () => getArticleList(Number(kbId), page, status, keyword, sourceType, processStatus), { keepPreviousData: true });

  const statusOptions: TableFilterOption<string>[] = Object.entries(ARTICLE_STATUS_KEYS).map(([value, key]) => ({ value, label: t(key) }));
  const sourceOptions: TableFilterOption<number>[] = Object.entries(ARTICLE_SOURCE_KEYS).map(([value, key]) => ({ value: Number(value), label: t(key) }));
  const processOptions: TableFilterOption<string>[] = Object.entries(ARTICLE_PROCESS_KEYS).map(([value, key]) => ({ value, label: t(key) }));

  const isEmpty = !error && data && (data.items || []).length === 0;
  const hasFilters = status !== '-1' || sourceType !== 0 || processStatus !== '' || keyword !== '';
  const clearFilters = () => { setStatus('-1'); setSourceType(0); setProcessStatus(''); setKeyword(''); setPage(1); };

  return (
    <div className="min-w-0 overflow-hidden">
      <div className="flex justify-between items-center mb-5">
        <div className="flex items-center gap-3">
          <IconButton label={t('common.back')} onClick={() => router.push('/admin/knowledge')}><ChevronLeft /></IconButton>
          <PageTitle className="mb-0">{t('kb.articlesTitle')}</PageTitle>
        </div>
        <IconButton label={t('kb.newArticle')} onClick={() => router.push(`/admin/knowledge/${kbId}/new`)}><FilePlus /></IconButton>
      </div>
      {error && <InlineError />}
      <div className="mb-4">
        <ListSearchInput value={keyword} onDebouncedChange={(v) => { setKeyword(v); setPage(1); }} placeholder={t('kb.searchArticlesPlaceholder')} />
      </div>
      {isEmpty ? (
        <EmptyState
          icon={<FileText size={40} />}
          title={hasFilters ? t('kb.articleNoMatch') : t('kb.articleEmpty')}
          description={hasFilters ? t('ticket.adjustFilters') : t('kb.articleEmptyDesc')}
          onClearFilters={hasFilters ? clearFilters : undefined}
        />
      ) : (
        <>
          <DataTable
            columns={[
              { accessorKey: 'title', header: t('kb.colTitle'), cell: ({ row }) => <Link href={`/admin/knowledge/${kbId}/${row.original.id}`} className="text-[var(--color-accent)]">{row.original.title}</Link> },
              { id: 'source_type_text', meta: { width: '72px' }, header: () => <TableFilterHeader label={t('kb.colSource')} value={sourceType} options={sourceOptions} onChange={(v) => { setSourceType(v); setPage(1); }} />, cell: ({ row }) => <span className="text-fine">{row.original.source_type === 1 ? t('kb.sourceManualShort') : t('kb.sourceUploadShort')}</span> },
              { accessorKey: 'status', meta: { width: '88px' }, header: () => <TableFilterHeader label={t('kb.colStatus')} value={status} options={statusOptions} onChange={(v) => { setStatus(v); setPage(1); }} />, cell: ({ row }) => <StatusBadge type="article" status={row.original.status} /> },
              { accessorKey: 'process_status', meta: { width: '88px' }, header: () => <TableFilterHeader label={t('kb.colProcess')} value={processStatus} options={processOptions} onChange={(v) => { setProcessStatus(v); setPage(1); }} />, cell: ({ row }) => row.original.process_status ? <StatusBadge type="process" status={row.original.process_status} /> : '—' },
              { id: 'created_at', meta: { width: '120px' }, header: t('kb.colUpdatedAt'), cell: ({ row }) => formatDate(row.original.updated_at, locale) },
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
