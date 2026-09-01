'use client';

import useSWR from 'swr';
import { useParams, useRouter } from 'next/navigation';
import { useState } from 'react';
import {
  createKnowledgeCandidate,
  getAdminTicketDetail,
  updateTicketStatus,
  type TicketDetail,
} from '@/lib/api/ticket';
import { getKBList } from '@/lib/api/knowledge';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { Field } from '@/components/ui/form-field';
import { Card } from '@/components/ui/card';
import { StatusBadge } from '@/components/shared/StatusBadge';
import { formatDate } from '@/lib/date';
import { toast } from 'sonner';
import { Play, CheckCircle, XCircle, MessageSquare, Sparkles, ChevronLeft, Loader2 } from 'lucide-react';

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
  const router = useRouter();
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
    return <div className="flex justify-center py-10"><Loader2 className="animate-spin" /></div>;
  }

  return (
    <div className="max-w-content">
      <Button variant="ghost" size="icon" aria-label="返回" onClick={() => router.push('/admin/tickets')}><ChevronLeft /></Button>
      <h1 className="mb-2 text-display font-semibold text-[var(--color-ink)]">{ticket.title}</h1>
      <div className="mb-5 flex items-center gap-3">
        <StatusBadge type="ticket" status={ticket.status} />
        <span className="text-caption text-[var(--color-text-muted-48)]">
          {ticket.ticket_no} / 提交人 {ticket.submitter_name || '-'} / {formatDate(ticket.created_at)}
        </span>
        {ticket.tags && ticket.tags.length > 0 && (
          <span className="flex flex-wrap gap-1">
            {ticket.tags.map((t) => (
              <span key={t} className="px-2 py-0.5 text-fine rounded-[var(--radius-pill)] bg-[var(--color-pearl)] text-[var(--color-text-muted-80)]">{t}</span>
            ))}
          </span>
        )}
      </div>

      <Card className="mb-4">
        <p className="whitespace-pre-wrap">{ticket.description}</p>
      </Card>

      <div className="mb-5 flex flex-wrap gap-2">
        {ticket.status === 1 && (
          <Button size="lg" disabled={processing} onClick={() => handleAction('start')}>{processing ? <Loader2 className="animate-spin" size={18} /> : <Play size={18} />}开始处理</Button>
        )}
        {ticket.status === 2 && (
          <>
            <Button size="lg" disabled={processing} onClick={() => handleAction('resolve')}>{processing ? <Loader2 className="animate-spin" size={18} /> : <CheckCircle size={18} />}标记解决</Button>
            <Button variant="ghost" size="sm" disabled={processing} onClick={() => handleAction('request_info')}>{processing ? <Loader2 className="animate-spin" size={16} /> : <MessageSquare size={16} />}索要补充</Button>
          </>
        )}
        {(ticket.status === 1 || ticket.status === 2 || ticket.status === 3) && (
          <Button variant="destructive" size="lg" disabled={processing} onClick={() => handleAction('close')}>{processing ? <Loader2 className="animate-spin" size={18} /> : <XCircle size={18} />}关闭申告</Button>
        )}
      </div>

      {ticket.status === 2 && (
        <Card className="mb-4">
          <Field label="处理说明">
            <Textarea
              value={actionResult}
              onChange={(e) => setActionResult(e.target.value)}
              rows={2}
              placeholder="可选：填写处理结果；索要补充时必填"
            />
          </Field>
        </Card>
      )}

      <Card className="mb-5">
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
          <Button variant="ghost" size="sm" disabled={!kbId} onClick={handleCreateKnowledgeCandidate}><Sparkles size={16} />生成</Button>
        </div>
      </Card>

      {ticket.records && ticket.records.length > 0 && (
        <Card>
          <h2 className="mb-3 text-title font-semibold">处理记录</h2>
          {ticket.records.map((record) => (
            <div key={record.id} className="border-b border-[var(--color-divider-soft)] py-2 last:border-b-0">
              <span className="text-caption font-semibold">{actionLabel(record.action)}</span>
              <span className="ml-3 text-fine text-[var(--color-text-muted-48)]">{formatDate(record.created_at)}</span>
              <p className="mt-1 text-caption">{record.content}</p>
            </div>
          ))}
        </Card>
      )}
    </div>
  );
}
