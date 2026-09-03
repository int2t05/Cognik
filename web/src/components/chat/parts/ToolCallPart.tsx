'use client';
// ToolCallPart — 工具调用展示（对标 Vercel ai-elements/tool.tsx）。
// Collapsible 折叠 + 状态图标（Clock=运行中/CheckCircle=完成）。

import { useState } from 'react';
import { Wrench, Clock, CheckCircle, ChevronDown } from 'lucide-react';
import type { MessagePart } from '@/lib/types';

interface Props {
  part: Extract<MessagePart, { type: 'tool_call' }>;
}

export function ToolCallPart({ part }: Props) {
  const [open, setOpen] = useState(false);
  const isRunning = part.status === 'running';

  return (
    <details className="my-2 group" open={open}>
      <summary
        className="text-[13px] cursor-pointer select-none flex items-center gap-1.5 text-[var(--color-text-muted-48)] hover:text-[var(--color-ink)]"
        onClick={(e) => { e.preventDefault(); setOpen(!open); }}
      >
        {isRunning ? (
          <Clock size={14} className="animate-spin text-[var(--color-accent)]" />
        ) : (
          <CheckCircle size={14} className="text-green-500" />
        )}
        <Wrench size={12} />
        <span className="font-medium">{part.label || '工具调用'}</span>
        <ChevronDown size={12} className={`transition-transform ${open ? 'rotate-180' : ''}`} />
      </summary>
      <div className="mt-1.5 ml-6 p-2 rounded-md bg-[var(--color-canvas)] border border-[var(--color-hairline)] text-[12px] font-mono whitespace-pre-wrap break-all">
        {part.content || '(无参数)'}
      </div>
    </details>
  );
}
