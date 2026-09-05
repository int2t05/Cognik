'use client';
import useSWR, { mutate as globalMutate } from 'swr';
import { useState } from 'react';
import { useTranslations, useLocale } from 'next-intl';
import { getMessages, markAsRead, markAllRead } from '@/lib/api/message';
import { PAGE_SIZE } from '@/lib/api/constants';
import { DataTable } from '@/components/ui/data-table';
import { DataTablePagination } from '@/components/ui/data-table-pagination';
import { IconButton } from '@/components/ui/icon-button';
import { formatDate } from '@/lib/date';
import { useRouter } from 'next/navigation';
import { toast } from 'sonner';
import { PageTitle } from '@/components/shared/PageTitle';
import { InlineError } from '@/components/shared/InlineError';
import { EmptyState } from '@/components/shared/EmptyState';
import { TableFilterHeader, type TableFilterOption } from '@/components/shared/TableFilterHeader';
import { CheckCheck, Mail, ExternalLink, Eye } from 'lucide-react';

/** 消息类型 → i18n 键（message.type.*）；未知类型原样显示 */
const MESSAGE_TYPE_KEYS: Record<string, string> = {
  ticket_supplement: 'message.type.ticket_supplement',
  ticket_resolved: 'message.type.ticket_resolved',
  ticket_closed: 'message.type.ticket_closed',
  ticket_overdue: 'message.type.ticket_overdue',
  knowledge_approved: 'message.type.knowledge_approved',
  knowledge_rejected: 'message.type.knowledge_rejected',
  knowledge_article: 'message.type.knowledge_article',
};

/** 有有效跳转目标的消息类型 */
const NAVIGABLE_TYPES = new Set(['ticket']);

export default function MessagesPage() {
  const t = useTranslations();
  const locale = useLocale();
  const [page, setPage] = useState(1);
  const [type, setType] = useState('');
  const router = useRouter();
  const { data, error, mutate } = useSWR(`messages-${page}-${type}`, () => getMessages(page, type), { keepPreviousData: true });

  const typeLabel = (key: string) => MESSAGE_TYPE_KEYS[key] ? t(MESSAGE_TYPE_KEYS[key]) : key;
  const typeOptions: TableFilterOption<string>[] = [
    { value: '', label: t('common.all') },
    ...Object.keys(MESSAGE_TYPE_KEYS).map((value) => ({ value, label: typeLabel(value) })),
  ];

  const handleRead = async (id: number, relatedType: string, relatedId: number) => {
    try {
      await markAsRead(id);
      mutate();
      globalMutate('unread-count');
      if (relatedType === 'ticket') router.push(`/portal/tickets/${relatedId}`);
    } catch {
      toast.error(t('message.markReadFailed'));
    }
  };

  const handleMarkAll = async () => {
    try {
      const res = await markAllRead();
      toast.success(res.affected > 0 ? t('message.markedRead', { count: res.affected }) : t('message.noUnread'));
      mutate();
      globalMutate('unread-count');
    } catch {
      toast.error(t('message.operationFailed'));
    }
  };

  const messages = data?.items ?? [];
  const hasUnread = messages.some((m) => !m.is_read);
  const isEmpty = !error && data && messages.length === 0;
  const hasFilters = type !== '';
  const clearFilters = () => { setType(''); setPage(1); };

  return (
    <div className="min-w-0 overflow-hidden">
      <div className="flex items-center justify-between mb-5">
        <PageTitle className="mb-0">{t('message.title')}</PageTitle>
        {!isEmpty && (
          <IconButton variant="secondary" size="sm" onClick={handleMarkAll} disabled={!hasUnread}>
            <CheckCheck size={16} />{t('message.markAllRead')}
          </IconButton>
        )}
      </div>

      {error && <InlineError />}

      {isEmpty ? (
        <EmptyState icon={<Mail size={40} />} title={hasFilters ? t('message.noMatch') : t('message.empty')} onClearFilters={hasFilters ? clearFilters : undefined} />
      ) : (
        <>
          <DataTable
            columns={[
              { accessorKey: 'type', meta: { width: '88px' }, header: () => <TableFilterHeader label={t('message.colType')} value={type} options={typeOptions} onChange={(v) => { setType(v); setPage(1); }} />, cell: ({ row }) => <span className="text-fine text-[var(--color-text-muted-48)]">{typeLabel(row.original.type)}</span> },
              { accessorKey: 'title', meta: { width: '120px' }, header: t('message.colTitle'), cell: ({ row }) => <span className={row.original.is_read ? 'text-[var(--color-text-muted-80)]' : 'font-semibold'}>{row.original.title}</span> },
              { accessorKey: 'content', header: t('message.colContent'), cell: ({ row }) => <span className={row.original.is_read ? 'text-[var(--color-text-muted-48)]' : ''}>{row.original.content}</span> },
              { accessorKey: 'created_at', meta: { width: '120px' }, header: t('message.colTime'), cell: ({ row }) => <span className={row.original.is_read ? 'text-[var(--color-text-muted-48)]' : ''}>{formatDate(row.original.created_at, locale)}</span> },
              { id: 'actions', meta: { width: '60px' }, header: '', cell: ({ row }) =>
                !row.original.is_read ? (
                  <IconButton label={t('message.view')} onClick={() => handleRead(row.original.id, row.original.related_type, row.original.related_id)}><Eye /></IconButton>
                ) : NAVIGABLE_TYPES.has(row.original.related_type) ? (
                  <IconButton label={t('message.goTo')} onClick={() => {
                    if (row.original.related_type === 'ticket') router.push(`/portal/tickets/${row.original.related_id}`);
                  }}>
                    <ExternalLink />
                  </IconButton>
                ) : null
              },
            ]}
            data={messages}
            loading={!data && !error}
          />
          {data && <DataTablePagination page={page} pageSize={PAGE_SIZE} total={data.total} onChange={(p) => setPage(p)} />}
        </>
      )}
    </div>
  );
}
