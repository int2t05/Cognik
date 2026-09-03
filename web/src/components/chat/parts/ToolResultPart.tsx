'use client';
// ToolResultPart — 工具结果展示。
// Collapsible 折叠 + CodeBlock 输出。

import { useState } from 'react';
import { CheckCircle, ChevronDown } from 'lucide-react';
import type { MessagePart } from '@/lib/types';

interface Props {
  part: Extract<MessagePart, { type: 'tool_result' }>;
}

export function ToolResultPart({ part }: Props) {
  const [open, setOpen] = useState(false);
  const content = part.content || '(无输出)';

  return (
    <details className="my-1 group" open={open}>
      <summary
        className="text-[13px] cursor-pointer select-none flex items-center gap-1.5 text-[var(--color-text-muted-48)] hover:text-[var(--color-ink)]"
        onClick={(e) => { e.preventDefault(); setOpen(!open); }}
      >
        <CheckCircle size={12} className="text-green-500" />
        <span>{part.label || '工具'}结果</span>
        <ChevronDown size={12} className={`transition-transform ${open ? 'rotate-180' : ''}`} />
      </summary>
      <div className="mt-1.5 ml-6 p-2 rounded-md bg-[var(--color-canvas)] border border-[var(--color-hairline)] text-[12px] font-mono whitespace-pre-wrap break-all max-h-60 overflow-y-auto">
        {content}
      </div>
    </details>
  );
}
