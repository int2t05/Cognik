'use client';
import { useState, useEffect, type FormEvent } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import { createTicket } from '@/lib/api/ticket';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput, AppleTextarea } from '@/components/ui/AppleInput';
import { AppleCard } from '@/components/ui/AppleCard';
import { useToast } from '@/hooks/useToast';
import { PageTitle } from '@/components/shared/PageTitle';
import { Send } from 'lucide-react';

interface ChatContextData {
  session_id: number;
  question: string;
  answer: string;
  confidence: number;
}

export default function TicketSubmitPage() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const toast = useToast();

  const chatContextRaw = searchParams.get('chat_context');

  // 从 chat_context 解析预填数据：描述 = 用户原始问题，标题由用户自行填写
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [tags, setTags] = useState('');
  const [contactPhone, setContactPhone] = useState('');
  const [contactEmail, setContactEmail] = useState('');
  const [chatContext, setChatContext] = useState<ChatContextData | undefined>(undefined);
  const [submitting, setSubmitting] = useState(false);

  // 解析 chat_context 并预填标题和描述
  useEffect(() => {
    if (!chatContextRaw) return;
    try {
      const ctx: ChatContextData = JSON.parse(chatContextRaw);
      setChatContext(ctx);
      if (ctx.question) {
        if (!title) setTitle(ctx.question);
        if (!description) setDescription(ctx.question);
      }
    } catch {
      // URL 参数格式错误，静默忽略
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chatContextRaw]);

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
      toast.error(err instanceof Error ? err.message : '提交失败');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="max-w-form">
      <PageTitle>提交申告</PageTitle>
      <form onSubmit={handleSubmit}>
        <AppleCard className="mb-4">
          <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">问题信息</h2>
          <AppleInput label="申告标题" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="简要描述遇到的问题" />
          <AppleTextarea label="详细描述" value={description} onChange={(e) => setDescription(e.target.value)} rows={5} placeholder="请详细描述问题现象、发生时间、影响范围等" />
          <AppleInput label="标签（逗号分隔）" value={tags} onChange={(e) => setTags(e.target.value)} placeholder="如：网络,邮箱,VPN,紧急" />
        </AppleCard>
        <AppleCard className="mb-4">
          <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">联系信息</h2>
          <AppleInput label="联系电话" value={contactPhone} onChange={(e) => setContactPhone(e.target.value)} placeholder="方便运维人员联系您" />
          <AppleInput label="联系邮箱" value={contactEmail} onChange={(e) => setContactEmail(e.target.value)} placeholder="选填" />
        </AppleCard>
        <div className="flex gap-3">
          <AppleButton variant="pill" icon={<Send />} type="submit" loading={submitting}>提交申告</AppleButton>
          <AppleButton variant="ghost" type="button" onClick={() => router.push("/portal/tickets")}>取消</AppleButton>
        </div>
      </form>
    </div>
  );
}
