'use client';
// ReasoningPart — 思考过程展示。
// Collapsible 折叠（默认收起，点击展开），流式微光动画指示状态。

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { ChevronDown, Brain } from 'lucide-react';
import type { MessagePart } from '@/lib/types';

interface Props {
  part: Extract<MessagePart, { type: 'reasoning' }>;
  streaming?: boolean;
}

export function ReasoningPart({ part, streaming }: Props) {
  const t = useTranslations();
  const [open, setOpen] = useState(false);

  return (
    <details open={open} className="my-2 group">
      <summary
        className="text-[13px] cursor-pointer select-none flex items-center gap-1.5 text-[var(--color-text-muted-48)] hover:text-[var(--color-ink)]"
        onClick={(e) => { e.preventDefault(); setOpen(!open); }}
      >
        <Brain size={14} className={streaming ? 'animate-pulse text-[var(--color-accent)]' : ''} />
        <span className={streaming ? 'text-[var(--color-accent)]' : ''}>
          {streaming ? t('chat.thinking') : t('chat.reasoning')}
        </span>
        {streaming && (
          <span className="inline-flex gap-0.5">
            <span className="w-1 h-1 rounded-full bg-[var(--color-accent)] animate-pulse" style={{ animationDelay: '0ms' }} />
            <span className="w-1 h-1 rounded-full bg-[var(--color-accent)] animate-pulse" style={{ animationDelay: '200ms' }} />
            <span className="w-1 h-1 rounded-full bg-[var(--color-accent)] animate-pulse" style={{ animationDelay: '400ms' }} />
          </span>
        )}
        <ChevronDown size={12} className={`transition-transform ${open ? 'rotate-180' : ''}`} />
      </summary>
      <div className="mt-1.5 pl-5 text-[13px] text-[var(--color-text-muted-48)] whitespace-pre-wrap break-words border-l-2 border-[var(--color-hairline)] overflow-y-auto max-h-96">
        {part.content}
      </div>
    </details>
  );
}
