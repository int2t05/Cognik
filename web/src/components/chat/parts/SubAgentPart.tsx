'use client';
// SubAgentPart — 子 Agent 委托展示（dispatch_subagent / research / coder）。
// 折叠卡片：状态图标 + 名称 + 展开内容（活动日志 + 最终结果分离）。

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { Bot, ChevronDown, Loader2, CheckCircle, XCircle } from 'lucide-react';
import type { MessagePart } from '@/lib/types';

interface Props {
  part: Extract<MessagePart, { type: 'tool_call' }>;
  isStreaming?: boolean;
}

/** 子 Agent 名 → i18n 键（chat.subagent.*） */
const SUBAGENT_KEYS: Record<string, string> = {
  dispatch_subagent: 'chat.subagent.dispatch',
  research: 'chat.subagent.research',
  coder: 'chat.subagent.coder',
};

export function SubAgentPart({ part, isStreaming }: Props) {
  const t = useTranslations();
  const [open, setOpen] = useState(false);
  const isRunning = part.status === 'running' && isStreaming;
  const isError = part.status === 'error';
  const label = SUBAGENT_KEYS[part.label] ? t(SUBAGENT_KEYS[part.label]) : (part.label || t('chat.subagent.dispatch'));

  // 分离活动日志和最终结果（content 格式：活动\n--- result ---\n最终结果）
  const content = part.content || '';
  const resultIdx = content.indexOf('\n--- result ---\n');
  const activity = resultIdx >= 0 ? content.slice(0, resultIdx) : content;
  const result = resultIdx >= 0 ? content.slice(resultIdx + '\n--- result ---\n'.length) : '';

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
        {isRunning && <Loader2 size={13} className="animate-spin text-[var(--color-accent)] ml-auto" />}
        {!isRunning && !isError && <CheckCircle size={13} className="text-green-500 ml-auto" />}
        {isError && <XCircle size={13} className="text-red-500 ml-auto" />}
        <ChevronDown size={12} className={`transition-transform ${open ? 'rotate-180' : ''} ${isRunning ? 'ml-0' : 'ml-1'}`} />
      </button>
      {open && (
        <div className="border-t border-[var(--color-accent)]/20 px-3 py-2 space-y-2">
          {activity && (
            <div>
              <div className="text-[11px] text-[var(--color-text-muted-48)] mb-1">{t('chat.activityLog')}</div>
              <pre className="text-[12px] font-mono whitespace-pre-wrap break-all text-[var(--color-text-muted-48)] bg-[var(--color-canvas)] p-2 rounded text-left max-h-40 overflow-y-auto">{activity}</pre>
            </div>
          )}
          {result && (
            <div>
              <div className="text-[11px] text-[var(--color-text-muted-48)] mb-1">{t('chat.finalResult')}</div>
              <pre className="text-[12px] font-mono whitespace-pre-wrap break-all text-[var(--color-ink)] bg-[var(--color-canvas)] p-2 rounded text-left max-h-60 overflow-y-auto">{result}</pre>
            </div>
          )}
          {!activity && !result && (
            <div className="text-[12px] text-[var(--color-text-muted-48)] italic">{isRunning ? t('chat.tool.running') : t('chat.tool.empty')}</div>
          )}
        </div>
      )}
    </div>
  );
}
