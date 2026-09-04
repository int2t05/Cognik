'use client';
// SubAgentPart — 子 Agent 调用展示（research/coder 委托任务）。
// 对标 Claude Code 的 Agent 工具展开（子 Agent 开始/运行中/完成）。

import { useState } from 'react';
import { Bot, ChevronDown, Loader2, CheckCircle, XCircle } from 'lucide-react';
import type { MessagePart } from '@/lib/types';

interface Props {
  part: Extract<MessagePart, { type: 'tool_call' }> & { label: 'research' | 'coder' };
  isStreaming?: boolean;
}

const SUBAGENT_LABELS: Record<string, string> = {
  research: '探查助手',
  coder: '编码助手',
};

export function SubAgentPart({ part, isStreaming }: Props) {
  const [open, setOpen] = useState(false);
  const isRunning = part.status === 'running' && isStreaming;
  const isError = part.status === 'error';
  const label = SUBAGENT_LABELS[part.label] || part.label || '子代理';

  return (
    <div className="my-2 rounded-md border border-[var(--color-accent)]/30 overflow-hidden bg-[var(--color-accent)]/5">
      <button
        className="w-full flex items-center gap-2 px-3 py-2 text-[13px] hover:bg-[var(--color-accent)]/10 transition-colors"
        onClick={() => setOpen(!open)}
      >
        <div className="w-6 h-6 rounded-full bg-[var(--color-accent)]/15 flex items-center justify-center shrink-0">
          <Bot size={13} className="text-[var(--color-accent)]" />
        </div>
        <span className="font-medium text-[var(--color-ink)]">{label}</span>
        <span className="text-[var(--color-text-muted-48)] text-[12px]">委托任务</span>
        {isRunning && <Loader2 size={13} className="animate-spin text-[var(--color-accent)] ml-auto" />}
        {!isRunning && !isError && <CheckCircle size={13} className="text-green-500 ml-auto" />}
        {isError && <XCircle size={13} className="text-red-500 ml-auto" />}
        <ChevronDown size={12} className={`transition-transform ${open ? 'rotate-180' : ''} ${isRunning ? 'ml-0' : 'ml-1'}`} />
      </button>
      {open && part.content && (
        <div className="border-t border-[var(--color-accent)]/20 px-3 py-2 text-[12px] text-[var(--color-text-muted-48)] whitespace-pre-wrap break-words max-h-60 overflow-y-auto">
          {part.content}
        </div>
      )}
    </div>
  );
}
