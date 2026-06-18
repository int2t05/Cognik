'use client';
import useSWR from 'swr';
import { useParams, useRouter } from 'next/navigation';
import { useState } from 'react';
import { getAdminTicketDetail, updateTicketStatus, addTicketRecord, createKnowledgeCandidate, type TicketDetail } from '@/lib/api/ticket';
import { getKBList } from '@/lib/api/knowledge';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput, AppleTextarea } from '@/components/ui/AppleInput';
import { AppleCard } from '@/components/ui/AppleCard';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { useToast } from '@/hooks/useToast';
import styles from './page.module.css';

type Action = 'start' | 'request_info' | 'resolve' | 'close';

export default function AdminTicketDetailPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const toast = useToast();
  const { data: ticket, error, mutate } = useSWR(`admin-ticket-${id}`, () => getAdminTicketDetail(Number(id)));
  const { data: kbs } = useSWR('kb-list', getKBList);
  const [actionResult, setActionResult] = useState('');
  const [processing, setProcessing] = useState(false);
  const [kbId, setKbId] = useState<number>(0);

  const handleAction = async (action: Action) => {
    if (action === 'request_info' && !actionResult.trim()) { toast.error('请填写需要补充的信息'); return; }
    setProcessing(true);
    try {
      await updateTicketStatus(Number(id), action, actionResult || undefined);
      toast.success('操作成功');
      setActionResult('');
      mutate();
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '操作失败'); }
    finally { setProcessing(false); }
  };

  if (error) return <p className={styles.error}>加载失败</p>;
  if (!ticket) return <div className={styles.loading}>加载中...</div>;

  return (
    <div className={styles.wrapper}>
      <h1 className={styles.title}>{ticket.title}</h1>
      <div className={styles.meta}>
        <StatusBadge type="ticket" status={ticket.status} />
        <span className={styles.metaText}>{ticket.ticket_no} · 提交人: {ticket.submitter_name} · {formatDate(ticket.created_at)}</span>
      </div>

      <AppleCard className={styles.cardMb}><p className={styles.descPre}>{ticket.description}</p></AppleCard>

      <div className={styles.actionBar}>
        {ticket.status === 1 && <AppleButton onClick={() => handleAction('start')} loading={processing}>开始处理</AppleButton>}
        {ticket.status === 2 && <><AppleButton onClick={() => handleAction('resolve')} loading={processing}>标记解决</AppleButton><AppleButton variant="ghost" onClick={() => handleAction('request_info')} loading={processing}>索要补充</AppleButton></>}
        {(ticket.status === 1 || ticket.status === 2 || ticket.status === 3) && <AppleButton variant="utility" onClick={() => handleAction('close')} loading={processing}>关闭申告</AppleButton>}
      </div>

      {ticket.status === 2 && (
        <AppleCard className={styles.cardMb}>
          <AppleTextarea label="处理说明" value={actionResult} onChange={(e) => setActionResult(e.target.value)} rows={2} placeholder="可选：填写处理结果..." />
        </AppleCard>
      )}

      {/* 知识候选 */}
      <AppleCard className={styles.cardMbLg}>
        <h3 className={styles.sectionTitle}>生成知识候选</h3>
        <div className={styles.formRow}>
          <select value={kbId} onChange={(e) => setKbId(Number(e.target.value))} className={styles.select}>
            <option value={0}>选择知识库...</option>
            {(kbs || []).map((kb) => <option key={kb.id} value={kb.id}>{kb.name}</option>)}
          </select>
          <AppleButton variant="ghost" disabled={!kbId} onClick={async () => { try { await createKnowledgeCandidate(Number(id), kbId); toast.success('已生成知识候选'); } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '生成失败'); } }}>生成</AppleButton>
        </div>
      </AppleCard>

      {/* 处理记录 */}
      {ticket.records && ticket.records.length > 0 && (
        <AppleCard>
          <h3 className={styles.sectionTitle}>处理记录</h3>
          {ticket.records.map((r) => (
            <div key={r.id} className={styles.record}>
              <span className={styles.recordAction}>{r.action}</span>
              <span className={styles.recordDate}>{formatDate(r.created_at)}</span>
              <p className={styles.recordContent}>{r.content}</p>
            </div>
          ))}
        </AppleCard>
      )}
    </div>
  );
}
