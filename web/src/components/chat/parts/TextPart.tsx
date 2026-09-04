'use client';
// TextPart — 流式文本渲染（� markdown）。
// 流式中纯文本（避免每 token 重解析），完成后 markdown 渲染。

import { memo, lazy, Suspense } from 'react';
import type { MessagePart } from '@/lib/types';

const Markdown = lazy(() => import('@/components/shared/Markdown').then((m) => ({ default: m.Markdown })));

interface Props {
  part: Extract<MessagePart, { type: 'text' }>;
  streaming?: boolean;
}

function TextPartBase({ part, streaming }: Props) {
  if (!part.content) {
    return streaming ? <span className="animate-pulse">▋</span> : null;
  }
  // 流式中纯文本（避免每 token 重解析 markdown），完成后 markdown 渲染
  if (streaming) return <span className="whitespace-pre-wrap">{part.content}</span>;
  return (
    <Suspense fallback={<span className="whitespace-pre-wrap">{part.content}</span>}>
      <Markdown content={part.content} />
    </Suspense>
  );
}

export const TextPart = memo(TextPartBase, (prev, next) =>
  prev.part.content === next.part.content && prev.streaming === next.streaming
);
