'use client';
import useSWR from 'swr';
import { useParams, useRouter } from 'next/navigation';
import { getTicketDetail, supplementTicket } from '@/lib/api/ticket';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleTextarea } from '@/components/ui/AppleInput';
import { AppleCard } from '@/components/ui/AppleCard';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { useToast } from '@/hooks/useToast';
import { useState } from 'react';
import { ArrowLeft, Send } from 'lucide-react';

/** 申告状态：需补充信息 */
const TICKET_STATUS_NEED_SUPPLEMENT = 3;

export default function TicketDetailPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { data: ticket, error } = useSWR(`portal-ticket-${id}`, () => getTicketDetail(Number(id)));
  const [supplement, setSupplement] = useState('');
  const [sending, setSending] = useState(false);
  const toast = useToast();

  const handleSupplement = async () => {
    if (!supplement.trim()) return;
    setSending(true);
    try {
      await supplementTicket(Number(id), supplement);
      toast.success('补充信息已提交');
      setSupplement('');
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : '提交失败');
    } finally { setSending(false); }
  };

  if (error) return <p className="text-[var(--color-error)] text-caption py-10 text-center">加载失败，请刷新重试</p>;
  if (!ticket) return <div className="flex justify-center py-10"><AppleSpinner /></div>;

  return (
    <div className="max-w-content">
      <div className="flex items-center gap-3 mb-5">
        <AppleButton variant="ghost" icon={<ArrowLeft />} aria-label="返回" onClick={() => router.push('/portal/tickets')} />
      </div>
      <h1 className="text-display font-semibold text-[var(--color-ink)] mb-2">{ticket.title}</h1>
      <div className="flex gap-3 mb-5 items-center flex-wrap">
        <StatusBadge type="ticket" status={ticket.status} />
        <span className="text-caption text-[var(--color-text-muted-48)]">{ticket.ticket_no}</span>
        <span className="text-caption text-[var(--color-text-muted-48)]">提交于 {formatDate(ticket.created_at)}</span>
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
    </div>
  );
}
