'use client';
import useSWR from 'swr';
import { useParams, useRouter } from 'next/navigation';
import { useTranslations, useLocale } from 'next-intl';
import { getTicketDetail, supplementTicket, updateTicket, withdrawTicket } from '@/lib/api/ticket';
import { IconButton } from '@/components/ui/icon-button';
import { ConfirmDialog } from '@/components/shared/ConfirmDialog';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
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
import { useState } from 'react';
import { ChevronLeft, Send, Pencil, Save, Loader2, Ban } from 'lucide-react';

/** 工单状态：需补充信息 */
const TICKET_STATUS_NEED_SUPPLEMENT = 3;

/** 可编辑的状态：待处理(1)、处理中(2) */
const canEdit = (status: number) => status === 1 || status === 2;

export default function TicketDetailPage() {
  const t = useTranslations();
  const locale = useLocale();
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { data: ticket, error, mutate } = useSWR(`portal-ticket-${id}`, () => getTicketDetail(Number(id)));
  const [supplement, setSupplement] = useState('');
  const [sending, setSending] = useState(false);
  const [withdrawConfirm, setWithdrawConfirm] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editTitle, setEditTitle] = useState('');
  const [editDesc, setEditDesc] = useState('');
  const [editTags, setEditTags] = useState('');
  const [editPhone, setEditPhone] = useState('');
  const [editEmail, setEditEmail] = useState('');

  const handleSupplement = async () => {
    if (!supplement.trim()) return;
    setSending(true);
    try {
      await supplementTicket(Number(id), supplement);
      toast.success(t('ticket.supplementSent'));
      setSupplement('');
      mutate();
    } catch (err: unknown) {
      toast.error(translateError(err, t, t('ticket.submitFailed')));
    } finally { setSending(false); }
  };

  const startEdit = () => {
    if (!ticket) return;
    setEditTitle(ticket.title);
    setEditDesc(ticket.description);
    setEditTags((ticket.tags || []).join(', '));
    setEditPhone(ticket.contact_phone);
    setEditEmail(ticket.contact_email || '');
    setEditing(true);
  };

  const handleSave = async () => {
    if (!editTitle.trim()) { toast.error(t('ticket.titleRequired')); return; }
    setSending(true);
    try {
      const tagList = editTags.split(',').map((s) => s.trim()).filter(Boolean);
      await updateTicket(Number(id), {
        title: editTitle.trim(),
        description: editDesc,
        tags: tagList,
        contact_phone: editPhone,
        contact_email: editEmail,
      });
      toast.success(t('ticket.updated'));
      setEditing(false);
      mutate();
    } catch (err: unknown) {
      toast.error(translateError(err, t, t('ticket.updateFailed')));
    } finally { setSending(false); }
  };

  // 编辑/保存切换：未编辑时进入编辑，编辑中点击即保存
  const toggleEdit = () => editing ? handleSave() : startEdit();

  const handleWithdraw = async () => {
    setSending(true);
    try {
      await withdrawTicket(Number(id));
      toast.success(t('ticket.withdrawn'));
      setWithdrawConfirm(false);
      mutate();
    } catch (err: unknown) {
      toast.error(translateError(err, t, t('ticket.withdrawFailed')));
    } finally { setSending(false); }
  };

  if (error) return <InlineError fullPage />;
  if (!ticket) return <div className="flex justify-center py-10"><Loader2 className="animate-spin" /></div>;

  return (
    <div className="max-w-content">
      <div className="flex items-center gap-3 mb-5">
        <IconButton label={t('common.back')} onClick={() => router.push('/portal/tickets')}><ChevronLeft /></IconButton>
        {canEdit(ticket.status) && (
          <IconButton label={editing ? t('common.save') : t('common.edit')} disabled={sending} onClick={toggleEdit}>{sending ? <Loader2 className="animate-spin" /> : editing ? <Save /> : <Pencil />}</IconButton>
        )}
        {ticket.status === 1 && (
          <IconButton label={t('ticket.withdraw')} danger disabled={sending} onClick={() => setWithdrawConfirm(true)}><Ban /></IconButton>
        )}
      </div>

      {editing ? (
        <Card className="mb-5">
          <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">{t('ticket.editTitle')}</h2>
          <Field label={t('ticket.titleLabel')}><Input value={editTitle} onChange={(e) => setEditTitle(e.target.value)} placeholder={t('ticket.titleLabel')} /></Field>
          <Field label={t('ticket.descriptionLabel')}><Textarea rows={5} value={editDesc} onChange={(e) => setEditDesc(e.target.value)} placeholder={t('ticket.descriptionLabel')} /></Field>
          <Field label={t('ticket.tagsLabel')}><Input value={editTags} onChange={(e) => setEditTags(e.target.value)} placeholder={t('ticket.tagsPlaceholder')} /></Field>
          <Field label={t('ticket.phoneLabel')}><Input value={editPhone} onChange={(e) => setEditPhone(e.target.value)} placeholder={t('ticket.phoneLabel')} /></Field>
          <Field label={t('ticket.emailLabel')}><Input value={editEmail} onChange={(e) => setEditEmail(e.target.value)} placeholder={t('ticket.emailPlaceholder')} /></Field>
        </Card>
      ) : (
        <>
          <PageTitle>{ticket.title}</PageTitle>
          <div className="flex gap-3 mb-5 items-center flex-wrap">
            <StatusBadge type="ticket" status={ticket.status} />
            <span className="text-caption text-[var(--color-text-muted-48)]">{ticket.ticket_no}</span>
            <span className="text-caption text-[var(--color-text-muted-48)]">{t('ticket.submittedAt', { date: formatDate(ticket.created_at, locale) })}</span>
            {ticket.tags && ticket.tags.length > 0 && (
              <span className="flex flex-wrap gap-1">
                {ticket.tags.map((tag) => (
                  <Badge key={tag} variant="neutral">{tag}</Badge>
                ))}
              </span>
            )}
          </div>

          <Card className="mb-5">
            <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">{t('ticket.issueDesc')}</h2>
            <Markdown content={ticket.description} />
          </Card>

          {ticket.records && ticket.records.length > 0 && (
            <Card className="mb-5">
              <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">{t('ticket.records')}</h2>
              {ticket.records.map((r) => (
                <div key={r.id} className="py-3 border-b border-[var(--color-divider-soft)] last:border-b-0">
                  <div className="flex justify-between mb-1">
                    <span className="text-caption font-semibold text-[var(--color-text-muted-80)]">{r.action}</span>
                    <span className="text-fine text-[var(--color-text-muted-48)]">{formatDate(r.created_at, locale)}</span>
                  </div>
                  <p className="text-caption text-[var(--color-ink)]">{r.content}</p>
                </div>
              ))}
            </Card>
          )}

          {ticket.status === TICKET_STATUS_NEED_SUPPLEMENT && (
            <Card>
              <h2 className="text-title font-semibold mb-3 text-[var(--color-ink)]">{t('ticket.supplementInfo')}</h2>
              <Textarea value={supplement} onChange={(e) => setSupplement(e.target.value)} rows={3} placeholder={t('ticket.supplementPlaceholder')} />
              <IconButton size="sm" disabled={sending} onClick={handleSupplement}>{sending ? <Loader2 className="animate-spin" size={16} /> : <Send size={16} />}{t('common.submit')}</IconButton>
            </Card>
          )}
        </>
      )}
      <ConfirmDialog
        open={withdrawConfirm}
        onOpenChange={setWithdrawConfirm}
        title={t('ticket.withdraw')}
        message={t('ticket.withdrawMessage')}
        confirmLabel={t('ticket.withdraw')}
        onConfirm={handleWithdraw}
        loading={sending}
        danger
      />
    </div>
  );
}
