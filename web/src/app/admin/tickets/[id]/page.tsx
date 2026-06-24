'use client';

import useSWR from 'swr';
import { useParams } from 'next/navigation';
import { useState } from 'react';
import {
  createKnowledgeCandidate,
  getAdminTicketDetail,
  updateTicketStatus,
  type TicketDetail,
} from '@/lib/api/ticket';
import { getKBList } from '@/lib/api/knowledge';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleTextarea } from '@/components/ui/AppleInput';
import { AppleCard } from '@/components/ui/AppleCard';
import { AppleSpinner } from '@/components/ui/AppleSpinner';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { useToast } from '@/hooks/useToast';
import { Play, CheckCircle, XCircle, MessageSquare, Sparkles } from 'lucide-react';

type Action = 'start' | 'request_info' | 'resolve' | 'close';

function actionLabel(action: string) {
  const labels: Record<string, string> = {
    create: '创建申告',
    start: '开始处理',
    request_info: '要求补充',
    supplement: '补充信息',
    resolve: '标记解决',
    close: '关闭申告',
  };
  return labels[action] || action;
}

export default function AdminTicketDetailPage() {
  const { id } = useParams<{ id: string }>();
  const ticketID = Number(id);
  const toast = useToast();
  const { data: ticket, error, mutate } = useSWR<TicketDetail>(`admin-ticket-${id}`, () => getAdminTicketDetail(ticketID));
  const { data: kbs } = useSWR('kb-list', getKBList);
  const [actionResult, setActionResult] = useState('');
  const [processing, setProcessing] = useState(false);
  const [kbId, setKbId] = useState<number>(0);

  const handleAction = async (action: Action) => {
    if (action === 'request_info' && !actionResult.trim()) {
      toast.error('请填写需要补充的信息');
      return;
    }

    setProcessing(true);
    try {
      await updateTicketStatus(ticketID, action, actionResult || undefined);
      toast.success('操作成功');
      setActionResult('');
      mutate();
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : '操作失败');
    } finally {
      setProcessing(false);
    }
  };

  const handleCreateKnowledgeCandidate = async () => {
    if (!kbId) return;
    try {
      await createKnowledgeCandidate(ticketID, kbId);
      toast.success('已生成知识候选');
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : '生成失败');
    }
  };

  if (error) {
    return <p className="text-[var(--color-error)] text-caption py-10 text-center">加载失败，请刷新重试</p>;
  }
  if (!ticket) {
    return <div className="flex justify-center py-10"><AppleSpinner /></div>;
  }

  return (
    <div className="max-w-content">
      <h1 className="mb-2 text-display font-semibold text-[var(--color-ink)]">{ticket.title}</h1>
      <div className="mb-5 flex items-center gap-3">
        <StatusBadge type="ticket" status={ticket.status} />
        <span className="text-caption text-[var(--color-text-muted-48)]">
          {ticket.ticket_no} / 提交人 {ticket.submitter_name || '-'} / {formatDate(ticket.created_at)}
        </span>
      </div>

      <AppleCard className="mb-4">
        <p className="whitespace-pre-wrap">{ticket.description}</p>
      </AppleCard>

      <div className="mb-5 flex flex-wrap gap-2">
        {ticket.status === 1 && (
          <AppleButton onClick={() => handleAction('start')} loading={processing}>
            <Play size={16} /> 开始处理
          </AppleButton>
        )}
        {ticket.status === 2 && (
          <>
            <AppleButton onClick={() => handleAction('resolve')} loading={processing}>
              <CheckCircle size={16} /> 标记解决
            </AppleButton>
            <AppleButton variant="ghost" onClick={() => handleAction('request_info')} loading={processing}>
              <MessageSquare size={16} /> 索要补充
            </AppleButton>
          </>
        )}
        {(ticket.status === 1 || ticket.status === 2 || ticket.status === 3) && (
          <AppleButton variant="utility" onClick={() => handleAction('close')} loading={processing}>
            <XCircle size={16} /> 关闭申告
          </AppleButton>
        )}
      </div>

      {ticket.status === 2 && (
        <AppleCard className="mb-4">
          <AppleTextarea
            label="处理说明"
            value={actionResult}
            onChange={(e) => setActionResult(e.target.value)}
            rows={2}
            placeholder="可选：填写处理结果；索要补充时必填"
          />
        </AppleCard>
      )}

      <AppleCard className="mb-5">
        <h2 className="mb-3 text-title font-semibold">生成知识候选</h2>
        <div className="flex items-end gap-3">
          <select
            value={kbId}
            onChange={(e) => setKbId(Number(e.target.value))}
            aria-label="选择知识库"
            className="cursor-pointer rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] px-4 py-2 text-body text-[var(--color-ink)]"
          >
            <option value={0}>选择知识库...</option>
            {(kbs || []).map((kb) => (
              <option key={kb.id} value={kb.id}>
                {kb.name}
              </option>
            ))}
          </select>
          <AppleButton variant="ghost" disabled={!kbId} onClick={handleCreateKnowledgeCandidate}>
            <Sparkles size={16} /> 生成
          </AppleButton>
        </div>
      </AppleCard>

      {ticket.records && ticket.records.length > 0 && (
        <AppleCard>
          <h2 className="mb-3 text-title font-semibold">处理记录</h2>
          {ticket.records.map((record) => (
            <div key={record.id} className="border-b border-[var(--color-divider-soft)] py-2 last:border-b-0">
              <span className="text-caption font-semibold">{actionLabel(record.action)}</span>
              <span className="ml-3 text-fine text-[var(--color-text-muted-48)]">{formatDate(record.created_at)}</span>
              <p className="mt-1 text-caption">{record.content}</p>
            </div>
          ))}
        </AppleCard>
      )}
    </div>
  );
}
