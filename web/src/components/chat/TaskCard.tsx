'use client';
// TaskCard — 异步任务卡片展示。
// 展示后台任务状态：pending/running/completed/failed/cancelled。

import { Loader2, CheckCircle, XCircle, Clock, AlertTriangle } from 'lucide-react';

export interface TaskInfo {
  id: number;
  type: string;
  status: string;
  input: string;
  output: string;
  error: string;
  created_at: string;
}

interface Props {
  task: TaskInfo;
}

const STATUS_CONFIG: Record<string, { icon: typeof Clock; color: string; label: string }> = {
  pending:   { icon: Clock, color: 'text-[var(--color-text-muted-48)]', label: '等待中' },
  running:   { icon: Loader2, color: 'text-[var(--color-accent)]', label: '执行中' },
  completed: { icon: CheckCircle, color: 'text-green-500', label: '已完成' },
  failed:    { icon: XCircle, color: 'text-red-500', label: '失败' },
  cancelled: { icon: AlertTriangle, color: 'text-amber-500', label: '已取消' },
};

export function TaskCard({ task }: Props) {
  const cfg = STATUS_CONFIG[task.status] || STATUS_CONFIG.pending;
  const Icon = cfg.icon;
  const isSpinning = task.status === 'running';

  let question = '';
  try { question = JSON.parse(task.input)?.question || ''; } catch {}

  let answer = '';
  if (task.output) {
    try { answer = JSON.parse(task.output)?.answer || ''; } catch {}
  }

  return (
    <div className="my-2 rounded-md border border-[var(--color-hairline)] overflow-hidden">
      <div className="flex items-center gap-2 px-3 py-2 text-[13px]">
        <Icon size={14} className={`${cfg.color} shrink-0 ${isSpinning ? 'animate-spin' : ''}`} />
        <span className="font-medium text-[var(--color-ink)]">任务 #{task.id}</span>
        <span className={`text-[12px] ${cfg.color}`}>{cfg.label}</span>
        <span className="text-[var(--color-text-muted-48)] text-[12px] truncate ml-auto">{question}</span>
      </div>
      {answer && task.status === 'completed' && (
        <div className="border-t border-[var(--color-hairline)] px-3 py-2 text-[12px] text-[var(--color-text-muted-48)] whitespace-pre-wrap break-words max-h-40 overflow-y-auto">
          {answer}
        </div>
      )}
      {task.error && task.status === 'failed' && (
        <div className="border-t border-red-200 px-3 py-2 text-[12px] text-red-500">
          {task.error}
        </div>
      )}
    </div>
  );
}
