'use client';
import { useState, useMemo, type FormEvent } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import { createTicket } from '@/lib/api/ticket';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Field } from '@/components/ui/form-field';
import { Card } from '@/components/ui/card';
import { toast } from 'sonner';
import { errorMessage } from '@/lib/api/error';
import { PageTitle } from '@/components/shared/PageTitle';
import { Send, Loader2 } from 'lucide-react';

interface ChatContextData {
  session_id: number;
  question: string;
  answer: string;
  confidence: number;
}

export default function TicketSubmitPage() {
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
    if (!title.trim()) { toast.error('请输入申告标题'); return; }

    setSubmitting(true);
    try {
      const tagList = tags.split(',').map((s) => s.trim()).filter(Boolean);
      await createTicket({
        title: title.trim(), description,
        tags: tagList,
        contact_phone: contactPhone || '—',
        contact_email: contactEmail, chat_context: chatContext,
      });
      toast.success('申告提交成功');
      router.push('/portal/tickets');
    } catch (err: unknown) {
      toast.error(errorMessage(err, '提交失败'));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="max-w-form">
      <PageTitle>提交申告</PageTitle>
      <form onSubmit={handleSubmit}>
        <Card className="mb-4">
          <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">问题信息</h2>
          <Field label="申告标题" required><Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="简要描述遇到的问题" /></Field>
          <Field label="详细描述" required><Textarea rows={5} value={description} onChange={(e) => setDescription(e.target.value)} placeholder="请详细描述问题现象、发生时间、影响范围等" /></Field>
          <Field label="标签（逗号分隔）"><Input value={tags} onChange={(e) => setTags(e.target.value)} placeholder="如：网络,邮箱,VPN,紧急" /></Field>
        </Card>
        <Card className="mb-4">
          <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">联系信息</h2>
          <Field label="联系电话" required><Input value={contactPhone} onChange={(e) => setContactPhone(e.target.value)} placeholder="方便运维人员联系您" /></Field>
          <Field label="联系邮箱"><Input value={contactEmail} onChange={(e) => setContactEmail(e.target.value)} placeholder="选填" /></Field>
        </Card>
        <div className="flex gap-3">
          <Button size="lg" type="submit" disabled={submitting}>{submitting ? <Loader2 className="animate-spin" size={18} /> : <Send size={18} />}提交申告</Button>
          <Button variant="ghost" size="sm" type="button" onClick={() => router.push("/portal/tickets")}>取消</Button>
        </div>
      </form>
    </div>
  );
}
