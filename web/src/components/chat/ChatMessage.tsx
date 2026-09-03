'use client';
// ChatMessage — 消息渲染（对标 shadcn-ui/chatbot-template chat-message.tsx）。
// switch(part.type) 分发到对应 part 组件。

import { memo } from 'react';
import { Bot, User, AlertCircle } from 'lucide-react';
import type { ChatMessage as ChatMessageType } from '@/lib/types';
import { TextPart } from './parts/TextPart';
import { ReasoningPart } from './parts/ReasoningPart';
import { ToolCallPart } from './parts/ToolCallPart';
import { ToolResultPart } from './parts/ToolResultPart';

interface Props {
  message: ChatMessageType;
  isStreaming?: boolean;
}

function ChatMessageBase({ message, isStreaming = false }: Props) {
  const isUser = message.role === 'user';
  const isError = message.status === 'error';
  const isCancelled = message.status === 'cancelled';

  return (
    <div className={`flex gap-3 mb-5 ${isUser ? 'justify-end' : 'justify-start'}`}>
      {!isUser && (
        <div className="w-8 h-8 rounded-full bg-[var(--color-accent)]/10 flex items-center justify-center shrink-0">
          <Bot size={16} className="text-[var(--color-accent)]" />
        </div>
      )}

      <div className={`px-4 py-3 text-body leading-relaxed ${
        isUser
          ? 'max-w-[70%] bg-zinc-200 dark:bg-zinc-700 text-[var(--color-ink)] rounded-[var(--radius-lg)] whitespace-pre-wrap'
          : 'w-full bg-[var(--color-canvas)] text-[var(--color-ink)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)]'
      } ${isError ? 'border-red-300' : ''}`}>
        {/* parts 数组分发渲染 */}
        {message.parts.map((part, i) => {
          const key = part.type === 'tool_call' || part.type === 'tool_result'
            ? `${part.type}-${part.id || i}`
            : `${part.type}-${i}`;
          switch (part.type) {
            case 'text':
              return <TextPart key={key} part={part} streaming={isStreaming} />;
            case 'reasoning':
              return <ReasoningPart key={key} part={part} streaming={isStreaming} />;
            case 'tool_call':
              return <ToolCallPart key={key} part={part} />;
            case 'tool_result':
              return <ToolResultPart key={key} part={part} />;
            default:
              return null;
          }
        })}

        {/* 空消息 + streaming 时显示加载指示器 */}
        {message.parts.length === 0 && isStreaming && (
          <span className="inline-flex items-center gap-1 text-[13px] text-[var(--color-text-muted-48)]">
            <span className="w-1.5 h-1.5 rounded-full bg-[var(--color-accent)] animate-pulse" />
            <span className="w-1.5 h-1.5 rounded-full bg-[var(--color-accent)] animate-pulse" style={{ animationDelay: '200ms' }} />
            <span className="w-1.5 h-1.5 rounded-full bg-[var(--color-accent)] animate-pulse" style={{ animationDelay: '400ms' }} />
          </span>
        )}

        {/* 错误状态 */}
        {isError && (
          <div className="flex items-center gap-1.5 text-red-500 text-[13px] mt-2 pt-2 border-t border-red-200">
            <AlertCircle size={14} />
            <span>{message.error || '生成失败'}</span>
          </div>
        )}

        {/* 取消状态 */}
        {isCancelled && (
          <div className="text-[13px] text-[var(--color-text-muted-48)] mt-2 pt-2 border-t border-[var(--color-hairline)]">
            已停止
          </div>
        )}
      </div>

      {isUser && (
        <div className="w-8 h-8 rounded-full bg-[var(--color-hairline)] flex items-center justify-center shrink-0">
          <User size={16} className="text-[var(--color-text-muted-48)]" />
        </div>
      )}
    </div>
  );
}

export const ChatMessage = memo(ChatMessageBase, (prev, next) =>
  prev.message === next.message && prev.isStreaming === next.isStreaming
);
