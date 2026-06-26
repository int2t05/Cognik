'use client';
import useSWR from 'swr';
import { useParams, useRouter } from 'next/navigation';
import { getTicketDetail, supplementTicket, updateTicket } from '@/lib/api/ticket';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput, AppleTextarea } from '@/components/ui/AppleInput';
import { AppleCard } from '@/components/ui/AppleCard';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { useToast } from '@/hooks/useToast';
import { useState } from 'react';
import { ChevronLeft, Send, Pencil, X, Check } from 'lucide-react';

/** 申告状态：需补充信息 */
const TICKET_STATUS_NEED_SUPPLEMENT = 3;

/** 可编辑的状态：待处理(1)、处理中(2) */
const canEdit = (status: number) => status === 1 || status === 2;

export default function TicketDetailPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { data: ticket, error, mutate } = useSWR(`portal-ticket-${id}`, () => getTicketDetail(Number(id)));
  const [supplement, setSupplement] = useState('');
  const [sending, setSending] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editTitle, setEditTitle] = useState('');
  const [editDesc, setEditDesc] = useState('');
  const [editTags, setEditTags] = useState('');
  const [editPhone, setEditPhone] = useState('');
  const [editEmail, setEditEmail] = useState('');
  const toast = useToast();

  const handleSupplement = async () => {
    if (!supplement.trim()) return;
    setSending(true);
    try {
      await supplementTicket(Number(id), supplement);
      toast.success('补充信息已提交');
      setSupplement('');
      mutate();
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : '提交失败');
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
    if (!editTitle.trim()) { toast.error('标题不能为空'); return; }
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
      toast.success('申告已更新');
      setEditing(false);
      mutate();
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : '更新失败');
    } finally { setSending(false); }
  };

  if (error) return <p className="text-[var(--color-error)] text-caption py-10 text-center">加载失败，请刷新重试</p>;
  if (!ticket) return <div className="flex justify-center py-10"><AppleSpinner /></div>;

  return (
    <div className="max-w-content">
      <div className="flex items-center gap-3 mb-5">
        <AppleButton variant="ghost" icon={<ChevronLeft />} aria-label="返回" onClick={() => router.push('/portal/tickets')} />
        {canEdit(ticket.status) && !editing && (
          <AppleButton variant="ghost" icon={<Pencil />} onClick={startEdit}>编辑</AppleButton>
        )}
      </div>

      {editing ? (
        <AppleCard className="mb-5">
          <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">编辑申告</h2>
          <AppleInput label="标题" value={editTitle} onChange={(e) => setEditTitle(e.target.value)} placeholder="申告标题" />
          <AppleTextarea label="详细描述" value={editDesc} onChange={(e) => setEditDesc(e.target.value)} rows={5} placeholder="详细描述" />
          <AppleInput label="标签（逗号分隔）" value={editTags} onChange={(e) => setEditTags(e.target.value)} placeholder="如：网络,邮箱,VPN" />
          <AppleInput label="联系电话" value={editPhone} onChange={(e) => setEditPhone(e.target.value)} placeholder="联系电话" />
          <AppleInput label="联系邮箱" value={editEmail} onChange={(e) => setEditEmail(e.target.value)} placeholder="选填" />
          <div className="flex gap-2 mt-4">
            <AppleButton variant="pill" icon={<Check />} onClick={handleSave} loading={sending}>保存</AppleButton>
            <AppleButton variant="ghost" icon={<X />} onClick={() => setEditing(false)}>取消</AppleButton>
          </div>
        </AppleCard>
      ) : (
        <>
          <h1 className="text-display font-semibold text-[var(--color-ink)] mb-2">{ticket.title}</h1>
          <div className="flex gap-3 mb-5 items-center flex-wrap">
            <StatusBadge type="ticket" status={ticket.status} />
            <span className="text-caption text-[var(--color-text-muted-48)]">{ticket.ticket_no}</span>
            <span className="text-caption text-[var(--color-text-muted-48)]">提交于 {formatDate(ticket.created_at)}</span>
            {ticket.tags && ticket.tags.length > 0 && (
              <span className="flex flex-wrap gap-1">
                {ticket.tags.map((t) => (
                  <span key={t} className="px-2 py-0.5 text-fine rounded-[var(--radius-pill)] bg-[var(--color-pearl)] text-[var(--color-text-muted-80)]">{t}</span>
                ))}
              </span>
            )}
          </div>

          <AppleCard className="mb-5">
            <h2 className="text-title font-semibold mb-3 text-[var(--color-ink)]">问题描述</h2>
            <p className="text-body text-[var(--color-ink)] leading-relaxed whitespace-pre-wrap">{ticket.description}</p>
          </AppleCard>

          {ticket.records && ticket.records.length > 0 && (
            <AppleCard className="mb-5">
              <h2 className="text-title font-semibold mb-4 text-[var(--color-ink)]">处理记录</h2>
              {ticket.records.map((r) => (
                <div key={r.id} className="py-3 border-b border-[var(--color-divider-soft)] last:border-b-0">
                  <div className="flex justify-between mb-1">
                    <span className="text-caption font-semibold text-[var(--color-text-muted-80)]">{r.action}</span>
                    <span className="text-fine text-[var(--color-text-muted-48)]">{formatDate(r.created_at)}</span>
                  </div>
                  <p className="text-caption text-[var(--color-ink)]">{r.content}</p>
                </div>
              ))}
            </AppleCard>
          )}

          {ticket.status === TICKET_STATUS_NEED_SUPPLEMENT && (
            <AppleCard>
              <h2 className="text-title font-semibold mb-3 text-[var(--color-ink)]">补充信息</h2>
              <AppleTextarea value={supplement} onChange={(e) => setSupplement(e.target.value)} rows={3} placeholder="请提供运维人员需要的补充信息..." />
              <AppleButton variant="pill" icon={<Send />} aria-label="提交补充" onClick={handleSupplement} loading={sending} />
            </AppleCard>
          )}
        </>
      )}
    </div>
  );
}
