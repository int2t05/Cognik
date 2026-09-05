'use client';
import { useState, useMemo, type FormEvent } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { createTicket } from '@/lib/api/ticket';
import { IconButton } from '@/components/ui/icon-button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Field } from '@/components/ui/form-field';
import { Card } from '@/components/ui/card';
import { toast } from 'sonner';
import { translateError } from '@/lib/api/error';
import { PageTitle } from '@/components/shared/PageTitle';
import { Send, Loader2 } from 'lucide-react';

interface ChatContextData {
  session_id: number;
  question: string;
  answer: string;
  confidence: number;
}

export default function TicketSubmitPage() {
  const t = useTranslations();
  const searchParams = useSearchParams();
  const router = useRouter();

  const chatContextRaw = searchParams.get('chat_context');

  // 从 chat_context 解析预填数据：描述 = 用户原始问题，标题由用户自行填写
  const chatContext = useMemo<ChatContextData | undefined>(() => {
    if (!chatContextRaw) return undefined;
    try { return JSON.parse(chatContextRaw) as ChatContextData; } catch { return undefined; }
  }, [chatContextRaw]);

  const [title, setTitle] = useState(chatContext?.question || '');
  const [description, setDescription] = useState(chatContext?.question || '');
  const [tags, setTags] = useState('');
  const [contactPhone, setContactPhone] = useState('');
  const [contactEmail, setContactEmail] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!title.trim()) { toast.error(t('ticket.titleRequired')); return; }

    setSubmitting(true);
    try {
      const tagList = tags.split(',').map((s) => s.trim()).filter(Boolean);
      await createTicket({
        title: title.trim(), description,
        tags: tagList,
        contact_phone: contactPhone || '—',
        contact_email: contactEmail, chat_context: chatContext,
      });
      toast.success(t('ticket.submitSuccess'));
      router.push('/portal/tickets');
    } catch (err: unknown) {
      toast.error(translateError(err, t, t('ticket.submitFailed')));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="max-w-form">
      <PageTitle>{t('nav.newTicket')}</PageTitle>
      <form onSubmit={handleSubmit}>
        <Card className="mb-4">
          <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">{t('ticket.issueInfo')}</h2>
          <Field label={t('ticket.titleLabel')} required><Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder={t('ticket.titlePlaceholder')} /></Field>
          <Field label={t('ticket.descriptionLabel')} required><Textarea rows={5} value={description} onChange={(e) => setDescription(e.target.value)} placeholder={t('ticket.descriptionPlaceholder')} /></Field>
          <Field label={t('ticket.tagsLabel')}><Input value={tags} onChange={(e) => setTags(e.target.value)} placeholder={t('ticket.tagsPlaceholder')} /></Field>
        </Card>
        <Card className="mb-4">
          <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">{t('ticket.contactInfo')}</h2>
          <Field label={t('ticket.phoneLabel')} required><Input value={contactPhone} onChange={(e) => setContactPhone(e.target.value)} placeholder={t('ticket.phonePlaceholder')} /></Field>
          <Field label={t('ticket.emailLabel')}><Input value={contactEmail} onChange={(e) => setContactEmail(e.target.value)} placeholder={t('ticket.emailPlaceholder')} /></Field>
        </Card>
        <div className="flex gap-3">
          <IconButton size="lg" type="submit" disabled={submitting}>{submitting ? <Loader2 className="animate-spin" size={18} /> : <Send size={18} />}{t('nav.newTicket')}</IconButton>

        </div>
      </form>
    </div>
  );
}
