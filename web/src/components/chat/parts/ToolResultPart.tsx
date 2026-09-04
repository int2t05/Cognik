'use client';
// ToolResultPart — 工具结果展示（仅在异常情况：tool_result 无对应 tool_call 时）。
// 正常流程 tool_result 已配对到 tool_call part（见 reducer.ts）。

import { CheckCircle } from 'lucide-react';
import type { MessagePart } from '@/lib/types';

interface Props {
  part: Extract<MessagePart, { type: 'tool_result' }>;
}

export function ToolResultPart({ part }: Props) {
  return (
    <div className="my-1 ml-6 p-2 rounded-md bg-[var(--color-canvas)] border border-[var(--color-hairline)] text-[12px] font-mono whitespace-pre-wrap break-all max-h-60 overflow-y-auto">
      <div className="flex items-center gap-1 text-green-500 mb-1">
        <CheckCircle size={12} />
        <span>{part.label || '工具'}结果</span>
      </div>
      {part.content || '(无输出)'}
    </div>
  );
}
