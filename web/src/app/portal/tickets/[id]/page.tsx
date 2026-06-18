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
import styles from './page.module.css';

/** 申告状态：需补充信息 */
const TICKET_STATUS_NEED_SUPPLEMENT = 3;

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

  if (error) return <p className={styles.error}>加载失败: {error.message}</p>;
  if (!ticket) return <div className={styles.loading}>加载中...</div>;

  return (
    <div className={styles.wrapper}>
      <h1 className={styles.title}>{ticket.title}</h1>
      <div className={styles.meta}>
        <StatusBadge type="ticket" status={ticket.status} />
        <span className={styles.metaText}>{ticket.ticket_no}</span>
        <span className={styles.metaText}>提交于 {formatDate(ticket.created_at)}</span>
      </div>

      <div className={styles.descCard}>
        <h2 className={styles.descTitle}>问题描述</h2>
        <p className={styles.descContent}>{ticket.description}</p>
      </div>

      {ticket.records && ticket.records.length > 0 && (
        <div className={styles.recordCard}>
          <h2 className={styles.recordTitle}>处理记录</h2>
          {ticket.records.map((r) => (
            <div key={r.id} className={styles.record}>
              <div className={styles.recordHeader}>
                <span className={styles.recordLabel}>{r.action}</span>
                <span className={styles.recordDate}>{formatDate(r.created_at)}</span>
              </div>
              <p className={styles.recordText}>{r.content}</p>
            </div>
          ))}
        </div>
      )}

      {ticket.status === TICKET_STATUS_NEED_SUPPLEMENT && (
        <div className={styles.supplementCard}>
          <h2 className={styles.supplementTitle}>补充信息</h2>
          <AppleTextarea value={supplement} onChange={(e) => setSupplement(e.target.value)} rows={3} placeholder="请提供运维人员需要的补充信息..." />
          <AppleButton onClick={handleSupplement} loading={sending}>提交补充</AppleButton>
        </div>
      )}
    </div>
  );
}
