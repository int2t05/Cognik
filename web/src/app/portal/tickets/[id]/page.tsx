'use client';
import useSWR from 'swr';
import { useParams } from 'next/navigation';
import { getTicketDetail, supplementTicket } from '@/lib/api/ticket';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleTextarea } from '@/components/ui/AppleInput';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { useToast } from '@/hooks/useToast';
import { useState } from 'react';

export default function TicketDetailPage() {
  const { id } = useParams<{ id: string }>();
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

  if (error) return <p style={{ color: 'var(--color-error)', padding: 40 }}>加载失败: {error.message}</p>;
  if (!ticket) return <div style={{ padding: 40, color: 'var(--text-muted-48)' }}>加载中...</div>;

  return (
    <div style={{ maxWidth: 720 }}>
      <h1 style={{ fontSize: 28, fontWeight: 600, color: 'var(--text-ink)', marginBottom: 8 }}>{ticket.title}</h1>
      <div style={{ display: 'flex', gap: 12, marginBottom: 24, alignItems: 'center' }}>
        <StatusBadge type="ticket" status={ticket.status} />
        <span style={{ fontSize: 13, color: 'var(--text-muted-48)' }}>{ticket.ticket_no}</span>
        <span style={{ fontSize: 13, color: 'var(--text-muted-48)' }}>提交于 {formatDate(ticket.created_at)}</span>
      </div>

      <div style={{ background: 'var(--bg-canvas)', borderRadius: 'var(--radius-lg)', border: '1px solid var(--hairline)', padding: 24, marginBottom: 24 }}>
        <h2 style={{ fontSize: 17, fontWeight: 600, marginBottom: 12, color: 'var(--text-ink)' }}>问题描述</h2>
        <p style={{ fontSize: 17, color: 'var(--text-ink)', lineHeight: 1.47, whiteSpace: 'pre-wrap' }}>{ticket.description}</p>
      </div>

      {ticket.records && ticket.records.length > 0 && (
        <div style={{ background: 'var(--bg-canvas)', borderRadius: 'var(--radius-lg)', border: '1px solid var(--hairline)', padding: 24, marginBottom: 24 }}>
          <h2 style={{ fontSize: 17, fontWeight: 600, marginBottom: 16, color: 'var(--text-ink)' }}>处理记录</h2>
          {ticket.records.map((r) => (
            <div key={r.id} style={{ padding: '12px 0', borderBottom: '1px solid var(--divider-soft)' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-muted-80)' }}>{r.action}</span>
                <span style={{ fontSize: 12, color: 'var(--text-muted-48)' }}>{formatDate(r.created_at)}</span>
              </div>
              <p style={{ fontSize: 14, color: 'var(--text-ink)' }}>{r.content}</p>
            </div>
          ))}
        </div>
      )}

      {ticket.status === 3 && (
        <div style={{ background: 'var(--bg-canvas)', borderRadius: 'var(--radius-lg)', border: '1px solid var(--hairline)', padding: 24 }}>
          <h2 style={{ fontSize: 17, fontWeight: 600, marginBottom: 12, color: 'var(--text-ink)' }}>补充信息</h2>
          <AppleTextarea value={supplement} onChange={(e) => setSupplement(e.target.value)} rows={3} placeholder="请提供运维人员需要的补充信息..." />
          <AppleButton onClick={handleSupplement} loading={sending}>提交补充</AppleButton>
        </div>
      )}
    </div>
  );
}
