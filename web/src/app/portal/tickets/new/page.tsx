'use client';
import { useState, type FormEvent } from 'react';
import { useSearchParams } from 'next/navigation';
import { useRouter } from 'next/navigation';
import { createTicket } from '@/lib/api/ticket';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput, AppleTextarea } from '@/components/ui/AppleInput';
import { useToast } from '@/hooks/useToast';

const IMPACT_OPTIONS = [
  { value: 1, label: '个人' },
  { value: 2, label: '部门' },
  { value: 3, label: '全公司' },
];

export default function TicketSubmitPage() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const toast = useToast();

  const chatContextRaw = searchParams.get('chat_context');
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [urgency, setUrgency] = useState(2);
  const [impactScope, setImpactScope] = useState(1);
  const [affectedSystems, setAffectedSystems] = useState('');
  const [contactPhone, setContactPhone] = useState('');
  const [contactEmail, setContactEmail] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!title.trim()) { toast.error('请输入申告标题'); return; }

    // 校验 chat_context JSON 结构
    let chatContext = undefined;
    if (chatContextRaw) {
      try { chatContext = JSON.parse(chatContextRaw); }
      catch { toast.error('聊天上下文数据格式错误'); return; }
    }

    setSubmitting(true);
    try {
      const systems = affectedSystems.split(',').map((s) => s.trim()).filter(Boolean);
      await createTicket({
        title: title.trim(), description, urgency, impact_scope: impactScope,
        affected_systems: systems, contact_phone: contactPhone || '—',
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
      <h1 className="text-hero font-semibold text-[var(--color-ink)] mb-6">提交申告</h1>
      <form onSubmit={handleSubmit}>
        <AppleInput label="申告标题" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="简要描述遇到的问题" />
        <AppleTextarea label="详细描述" value={description} onChange={(e) => setDescription(e.target.value)} rows={5} placeholder="请详细描述问题现象、发生时间、影响范围等" />
        <div className="flex gap-4 mb-4">
          <div className="flex-1">
            <label className="block text-caption font-medium text-[var(--color-ink)] mb-1.5">紧急程度</label>
            <select value={urgency} onChange={(e) => setUrgency(Number(e.target.value))} className="w-full px-3 py-2 text-body rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)]">
              <option value={1}>低 — 一般咨询</option>
              <option value={2}>中 — 影响工作</option>
              <option value={3}>高 — 紧急处理</option>
            </select>
          </div>
          <div className="flex-1">
            <label className="block text-caption font-medium text-[var(--color-ink)] mb-1.5">影响范围</label>
            <select value={impactScope} onChange={(e) => setImpactScope(Number(e.target.value))} className="w-full px-3 py-2 text-body rounded-[var(--radius-sm)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)]">
              {IMPACT_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
            </select>
          </div>
        </div>
        <AppleInput label="受影响系统（逗号分隔）" value={affectedSystems} onChange={(e) => setAffectedSystems(e.target.value)} placeholder="如：Exchange,Outlook,VPN" />
        <AppleInput label="联系电话" value={contactPhone} onChange={(e) => setContactPhone(e.target.value)} placeholder="方便运维人员联系您" />
        <AppleInput label="联系邮箱" value={contactEmail} onChange={(e) => setContactEmail(e.target.value)} placeholder="选填" />
        <div className="mt-6 flex gap-3">
          <AppleButton type="submit" loading={submitting}>提交申告</AppleButton>
          <AppleButton variant="ghost" type="button" onClick={() => router.push("/portal/tickets")}>取消</AppleButton>
        </div>
      </form>
    </div>
  );
}
