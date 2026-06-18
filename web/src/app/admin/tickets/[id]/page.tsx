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

  if (error) return <p className="text-[var(--color-error)] p-10">加载失败</p>;
  if (!ticket) return <div className="p-10 text-[var(--color-text-muted-48)]">加载中...</div>;

  return (
    <div className="max-w-[800px]">
      <h1 className="text-[28px] font-medium text-[var(--color-ink)] mb-2">{ticket.title}</h1>
      <div className="flex gap-3 mb-6 items-center">
        <StatusBadge type="ticket" status={ticket.status} />
        <span className="text-[13px] text-[var(--color-text-muted-48)]">{ticket.ticket_no} · 提交人: {ticket.submitter_name} · {formatDate(ticket.created_at)}</span>
      </div>

      <AppleCard className="mb-4"><p className="whitespace-pre-wrap">{ticket.description}</p></AppleCard>

      <div className="flex gap-2 mb-6 flex-wrap">
        {ticket.status === 1 && <AppleButton onClick={() => handleAction('start')} loading={processing}>开始处理</AppleButton>}
        {ticket.status === 2 && <><AppleButton onClick={() => handleAction('resolve')} loading={processing}>标记解决</AppleButton><AppleButton variant="ghost" onClick={() => handleAction('request_info')} loading={processing}>索要补充</AppleButton></>}
        {(ticket.status === 1 || ticket.status === 2 || ticket.status === 3) && <AppleButton variant="utility" onClick={() => handleAction('close')} loading={processing}>关闭申告</AppleButton>}
      </div>

      {ticket.status === 2 && (
        <AppleCard className="mb-4">
          <AppleTextarea label="处理说明" value={actionResult} onChange={(e) => setActionResult(e.target.value)} rows={2} placeholder="可选：填写处理结果..." />
        </AppleCard>
      )}

      {/* 知识候选 */}
      <AppleCard className="mb-6">
        <h3 className="text-[17px] font-medium mb-3">生成知识候选</h3>
        <div className="flex gap-3 items-end">
          <select value={kbId} onChange={(e) => setKbId(Number(e.target.value))} className="px-4 py-2 text-[15px] rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] cursor-pointer">
            <option value={0}>选择知识库...</option>
            {(kbs || []).map((kb) => <option key={kb.id} value={kb.id}>{kb.name}</option>)}
          </select>
          <AppleButton variant="ghost" disabled={!kbId} onClick={async () => { try { await createKnowledgeCandidate(Number(id), kbId); toast.success('已生成知识候选'); } catch (err: unknown) { toast.error(err instanceof Error ? err.message : '生成失败'); } }}>生成</AppleButton>
        </div>
      </AppleCard>

      {/* 处理记录 */}
      {ticket.records && ticket.records.length > 0 && (
        <AppleCard>
          <h3 className="text-[17px] font-medium mb-3">处理记录</h3>
          {ticket.records.map((r) => (
            <div key={r.id} className="py-2 border-b border-[var(--color-divider-soft)]">
              <span className="text-[13px] font-semibold">{r.action}</span>
              <span className="text-xs text-[var(--color-text-muted-48)] ml-3">{formatDate(r.created_at)}</span>
              <p className="text-sm mt-1">{r.content}</p>
            </div>
          ))}
        </AppleCard>
      )}
    </div>
  );
}
