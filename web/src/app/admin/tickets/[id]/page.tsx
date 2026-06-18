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

  if (error) return <p style={{ color: 'var(--color-error)', padding: 40 }}>加载失败</p>;
  if (!ticket) return <div style={{ padding: 40, color: 'var(--text-muted-48)' }}>加载中...</div>;

  return (
    <div style={{ maxWidth: 800 }}>
      <h1 style={{ fontSize: 28, fontWeight: 600, color: 'var(--text-ink)', marginBottom: 8 }}>{ticket.title}</h1>
      <div style={{ display: 'flex', gap: 12, marginBottom: 24, alignItems: 'center' }}>
        <StatusBadge type="ticket" status={ticket.status} />
        <span style={{ fontSize: 13, color: 'var(--text-muted-48)' }}>{ticket.ticket_no} · 提交人: {ticket.submitter_name} · {formatDate(ticket.created_at)}</span>
      </div>

      <AppleCard style={{ marginBottom: 16 }}><p style={{ whiteSpace: 'pre-wrap' }}>{ticket.description}</p></AppleCard>

      <div style={{ display: 'flex', gap: 8, marginBottom: 24, flexWrap: 'wrap' }}>
        {ticket.status === 1 && <AppleButton onClick={() => handleAction('start')} loading={processing}>开始处理</AppleButton>}
        {ticket.status === 2 && <><AppleButton onClick={() => handleAction('resolve')} loading={processing}>标记解决</AppleButton><AppleButton variant="ghost" onClick={() => handleAction('request_info')} loading={processing}>索要补充</AppleButton></>}
        {(ticket.status === 1 || ticket.status === 2 || ticket.status === 3) && <AppleButton variant="utility" onClick={() => handleAction('close')} loading={processing}>关闭申告</AppleButton>}
      </div>

      {ticket.status === 2 && (
        <AppleCard style={{ marginBottom: 16 }}>
          <AppleTextarea label="处理说明" value={actionResult} onChange={(e) => setActionResult(e.target.value)} rows={2} placeholder="可选：填写处理结果..." />
        </AppleCard>
      )}

      {/* 知识候选 */}
      <AppleCard style={{ marginBottom: 24 }}>
        <h3 style={{ fontSize: 17, fontWeight: 600, marginBottom: 12 }}>生成知识候选</h3>
        <div style={{ display: 'flex', gap: 12, alignItems: 'end' }}>
          <select value={kbId} onChange={(e) => setKbId(Number(e.target.value))} style={{ padding: '8px 12px', fontSize: 14, borderRadius: 'var(--radius-sm)', border: '1px solid var(--hairline)', background: 'var(--bg-canvas)', color: 'var(--text-ink)' }}>
            <option value={0}>选择知识库...</option>
            {(kbs || []).map((kb) => <option key={kb.id} value={kb.id}>{kb.name}</option>)}
          </select>
          <AppleButton variant="ghost" disabled={!kbId} onClick={async () => { try { await createKnowledgeCandidate(Number(id), kbId); toast.success('已生成知识候选'); } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '生成失败'); } }}>生成</AppleButton>
        </div>
      </AppleCard>

      {/* 处理记录 */}
      {ticket.records && ticket.records.length > 0 && (
        <AppleCard>
          <h3 style={{ fontSize: 17, fontWeight: 600, marginBottom: 12 }}>处理记录</h3>
          {ticket.records.map((r) => (
            <div key={r.id} style={{ padding: '8px 0', borderBottom: '1px solid var(--divider-soft)' }}>
              <span style={{ fontSize: 13, fontWeight: 600 }}>{r.action}</span>
              <span style={{ fontSize: 12, color: 'var(--text-muted-48)', marginLeft: 12 }}>{formatDate(r.created_at)}</span>
              <p style={{ fontSize: 14, marginTop: 4 }}>{r.content}</p>
            </div>
          ))}
        </AppleCard>
      )}
    </div>
  );
}
