'use client';
import { useState, type FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import { createTicket } from '@/lib/api/ticket';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput, AppleTextarea } from '@/components/ui/AppleInput';
import { useToast } from '@/hooks/useToast';

export default function TicketSubmitPage() {
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [urgency, setUrgency] = useState(2);
  const [contact, setContact] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const router = useRouter();
  const toast = useToast();

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!title.trim()) { toast.error('请输入申告标题'); return; }
    setSubmitting(true);
    try {
      await createTicket({ title: title.trim(), description, urgency, impact_scope: 1, contact_phone: contact || '—' });
      toast.success('申告提交成功');
      router.push('/portal/tickets');
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : '提交失败');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div style={{ maxWidth: 640 }}>
      <h1 style={{ fontSize: 28, fontWeight: 600, color: 'var(--text-ink)', marginBottom: 24 }}>提交申告</h1>
      <form onSubmit={handleSubmit}>
        <AppleInput label="申告标题" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="简要描述遇到的问题" />
        <AppleTextarea label="详细描述" value={description} onChange={(e) => setDescription(e.target.value)} rows={5} placeholder="请详细描述问题现象、发生时间、影响范围等" />
        <div style={{ marginBottom: 16 }}>
          <label style={{ display: 'block', fontSize: 14, fontWeight: 500, marginBottom: 6, color: 'var(--text-ink)' }}>紧急程度</label>
          <select value={urgency} onChange={(e) => setUrgency(Number(e.target.value))} style={{ padding: '8px 12px', fontSize: 17, borderRadius: 'var(--radius-sm)', border: '1px solid var(--hairline)', background: 'var(--bg-canvas)', color: 'var(--text-ink)' }}>
            <option value={1}>低 — 一般咨询</option>
            <option value={2}>中 — 影响工作</option>
            <option value={3}>高 — 紧急处理</option>
          </select>
        </div>
        <AppleInput label="联系电话" value={contact} onChange={(e) => setContact(e.target.value)} placeholder="方便运维人员联系您" />
        <div style={{ marginTop: 24, display: 'flex', gap: 12 }}>
          <AppleButton type="submit" loading={submitting}>提交申告</AppleButton>
          <AppleButton variant="ghost" type="button" onClick={() => router.back()}>取消</AppleButton>
        </div>
      </form>
    </div>
  );
}
