'use client';
// ToolCallPart — 工具调用展示（对标 Vercel ai-elements/tool.tsx）。
// Collapsible 卡片 + 状态图标（Clock=运行中 → CheckCircle=完成 → XCircle=错误）。
// 展示：工具名 + 参数 + 结果（配对后合并到 content）。

import { useState } from 'react';
import { Wrench, Clock, CheckCircle, XCircle, ChevronDown } from 'lucide-react';
import type { MessagePart } from '@/lib/types';

interface Props {
  part: Extract<MessagePart, { type: 'tool_call' }>;
}

// 工具名中文映射
const TOOL_LABELS: Record<string, string> = {
  bash: '执行命令',
  read_file: '读取文件',
  write_file: '写入文件',
  edit_file: '编辑文件',
  list_dir: '列出目录',
  glob: '搜索文件',
  grep: '搜索内容',
  mkdir: '创建目录',
};

export function ToolCallPart({ part }: Props) {
  const [open, setOpen] = useState(false);
  const isRunning = part.status === 'running';
  const isError = part.status === 'error';
  const toolLabel = TOOL_LABELS[part.label] || part.label || '工具调用';

  // 分离参数和结果（content 格式：args\n--- result ---\nresult）
  const content = part.content || '';
  const resultIdx = content.indexOf('\n--- result ---\n');
  const args = resultIdx >= 0 ? content.slice(0, resultIdx) : content;
  const result = resultIdx >= 0 ? content.slice(resultIdx + '\n--- result ---\n'.length) : '';

  return (
    <div className="my-2 rounded-md border border-[var(--color-hairline)] overflow-hidden">
      {/* 头部：状态图标 + 工具名 + 展开按钮 */}
      <button
        className="w-full flex items-center gap-2 px-3 py-2 text-[13px] text-[var(--color-text-muted-48)] hover:bg-[var(--color-canvas)] transition-colors"
        onClick={() => setOpen(!open)}
      >
        {isRunning ? (
          <Clock size={14} className="animate-spin text-[var(--color-accent)] shrink-0" />
        ) : isError ? (
          <XCircle size={14} className="text-red-500 shrink-0" />
        ) : (
          <CheckCircle size={14} className="text-green-500 shrink-0" />
        )}
        <Wrench size={12} className="shrink-0" />
        <span className="font-medium text-[var(--color-ink)]">{toolLabel}</span>
        <span className="text-[var(--color-text-muted-48)] text-[12px] truncate">{part.label}</span>
        <ChevronDown size={12} className={`ml-auto transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>

      {/* 展开内容：参数 + 结果 */}
      {open && (
        <div className="border-t border-[var(--color-hairline)] px-3 py-2 space-y-2">
          {args && (
            <div>
              <div className="text-[11px] text-[var(--color-text-muted-48)] mb-1">参数</div>
              <pre className="text-[12px] font-mono whitespace-pre-wrap break-all text-[var(--color-ink)] bg-[var(--color-canvas)] p-2 rounded text-left">{args}</pre>
            </div>
          )}
          {result && (
            <div>
              <div className="text-[11px] text-[var(--color-text-muted-48)] mb-1">结果</div>
              <pre className="text-[12px] font-mono whitespace-pre-wrap break-all text-[var(--color-ink)] bg-[var(--color-canvas)] p-2 rounded text-left max-h-60 overflow-y-auto">{result}</pre>
            </div>
          )}
          {!args && !result && (
            <div className="text-[12px] text-[var(--color-text-muted-48)] italic">{isRunning ? '执行中…' : '无内容'}</div>
          )}
        </div>
      )}
    </div>
  );
}
