'use client';

import useSWR from 'swr';
import { useParams, useRouter } from 'next/navigation';
import { useState } from 'react';
import { useTranslations, useLocale } from 'next-intl';
import {
  createKnowledgeCandidate,
  getAdminTicketDetail,
  updateTicketStatus,
  type TicketDetail,
} from '@/lib/api/ticket';
import { getKBList } from '@/lib/api/knowledge';
import { IconButton } from '@/components/ui/icon-button';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Field } from '@/components/ui/form-field';
import { Card } from '@/components/ui/card';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { PageTitle } from '@/components/shared/PageTitle';
import { Markdown } from '@/components/shared/Markdown';
import { InlineError } from '@/components/shared/InlineError';
import { Badge } from '@/components/ui/badge';
import { formatDate } from '@/lib/date';
import { toast } from 'sonner';
import { translateError } from '@/lib/api/error';
import { Play, CheckCircle, XCircle, MessageSquare, Sparkles, ChevronLeft, Loader2 } from 'lucide-react';

type Action = 'start' | 'request_info' | 'resolve' | 'close';

/** 工单操作 → i18n 键（ticket.action.*）；未知操作原样显示 */
const ACTION_KEYS: Record<string, string> = {
  create: 'ticket.action.create',
  start: 'ticket.action.start',
  request_info: 'ticket.action.request_info',
  supplement: 'ticket.action.supplement',
  resolve: 'ticket.action.resolve',
  close: 'ticket.action.close',
  withdraw: 'ticket.action.withdraw',
};

export default function AdminTicketDetailPage() {
  const t = useTranslations();
  const locale = useLocale();
  const { id } = useParams<{ id: string }>();
  const ticketID = Number(id);
  const router = useRouter();
  const { data: ticket, error, mutate } = useSWR<TicketDetail>(`admin-ticket-${id}`, () => getAdminTicketDetail(ticketID));
  const { data: kbs } = useSWR('kb-list', getKBList);
  const [actionResult, setActionResult] = useState('');
  const [processing, setProcessing] = useState(false);
  const [kbId, setKbId] = useState<number>(0);

  const actionLabel = (action: string) => ACTION_KEYS[action] ? t(ACTION_KEYS[action]) : action;

  const handleAction = async (action: Action) => {
    if (action === 'request_info' && !actionResult.trim()) {
      toast.error(t('ticket.fillSupplementInfo'));
      return;
    }

    setProcessing(true);
    try {
      await updateTicketStatus(ticketID, action, actionResult || undefined);
      toast.success(t('common.operationSuccess'));
      setActionResult('');
      mutate();
    } catch (err: unknown) {
      toast.error(translateError(err, t, t('common.operationFailed')));
    } finally {
      setProcessing(false);
    }
  };

  const handleCreateKnowledgeCandidate = async () => {
    if (!kbId) return;
    try {
      await createKnowledgeCandidate(ticketID, kbId);
      toast.success(t('ticket.knowledgeCandidateCreated'));
    } catch (err: unknown) {
      toast.error(translateError(err, t, t('ticket.generateFailed')));
    }
  };

  if (error) {
    return <InlineError fullPage />;
  }
  if (!ticket) {
    return <div className="flex justify-center py-10"><Loader2 className="animate-spin" /></div>;
  }

  return (
    <div className="max-w-content">
      <div className="flex items-center gap-3 mb-5">
        <IconButton label={t('common.back')} onClick={() => router.push('/admin/tickets')}><ChevronLeft /></IconButton>
        <PageTitle className="mb-0">{ticket.title}</PageTitle>
      </div>
      <div className="mb-5 flex items-center gap-3">
        <StatusBadge type="ticket" status={ticket.status} />
        <span className="text-caption text-[var(--color-text-muted-48)]">
          {ticket.ticket_no} · {t('ticket.submitter', { name: ticket.submitter_name || '-' })} · {formatDate(ticket.created_at, locale)}
        </span>
        {ticket.tags && ticket.tags.length > 0 && (
          <span className="flex flex-wrap gap-1">
            {ticket.tags.map((tag) => (
              <Badge key={tag} variant="neutral">{tag}</Badge>
            ))}
          </span>
        )}
      </div>

      <Card className="mb-4">
        <Markdown content={ticket.description} />
      </Card>

      <div className="mb-5 flex flex-wrap gap-2">
        {ticket.status === 1 && (
          <IconButton size="lg" disabled={processing} onClick={() => handleAction('start')}>{processing ? <Loader2 className="animate-spin" size={18} /> : <Play size={18} />}{t('ticket.action.start')}</IconButton>
        )}
        {ticket.status === 2 && (
          <>
            <IconButton size="lg" disabled={processing} onClick={() => handleAction('resolve')}>{processing ? <Loader2 className="animate-spin" size={18} /> : <CheckCircle size={18} />}{t('ticket.action.resolve')}</IconButton>
            <IconButton variant="ghost" size="sm" disabled={processing} onClick={() => handleAction('request_info')}>{processing ? <Loader2 className="animate-spin" size={16} /> : <MessageSquare size={16} />}{t('ticket.requestInfoBtn')}</IconButton>
          </>
        )}
        {(ticket.status === 1 || ticket.status === 2 || ticket.status === 3) && (
          <IconButton variant="destructive" size="lg" disabled={processing} onClick={() => handleAction('close')}>{processing ? <Loader2 className="animate-spin" size={18} /> : <XCircle size={18} />}{t('ticket.action.close')}</IconButton>
        )}
      </div>

      {ticket.status === 2 && (
        <Card className="mb-4">
          <Field label={t('ticket.processNote')}>
            <Textarea
              value={actionResult}
              onChange={(e) => setActionResult(e.target.value)}
              rows={2}
              placeholder={t('ticket.processNotePlaceholder')}
            />
          </Field>
        </Card>
      )}

      <Card className="mb-5">
        <h2 className="mb-3 text-title font-semibold">{t('ticket.generateKnowledgeCandidate')}</h2>
        <div className="flex items-end gap-3">
          <Select value={String(kbId)} onValueChange={(v) => setKbId(Number(v))}>
            <SelectTrigger aria-label={t('ticket.selectKb')} className="rounded-[var(--radius-pill)]">
              <SelectValue placeholder={t('ticket.selectKb')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="0">{t('ticket.selectKb')}</SelectItem>
              {(kbs || []).map((kb) => (
                <SelectItem key={kb.id} value={String(kb.id)}>
                  {kb.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <IconButton variant="ghost" size="sm" disabled={!kbId} onClick={handleCreateKnowledgeCandidate}><Sparkles size={16} />{t('ticket.generate')}</IconButton>
        </div>
      </Card>

      {ticket.records && ticket.records.length > 0 && (
        <Card>
          <h2 className="mb-3 text-title font-semibold">{t('ticket.records')}</h2>
          {ticket.records.map((record) => (
            <div key={record.id} className="border-b border-[var(--color-divider-soft)] py-2 last:border-b-0">
              <span className="text-caption font-semibold">{actionLabel(record.action)}</span>
              <span className="ml-3 text-fine text-[var(--color-text-muted-48)]">{formatDate(record.created_at, locale)}</span>
              <p className="mt-1 text-caption">{record.content}</p>
            </div>
          ))}
        </Card>
      )}
    </div>
  );
}
