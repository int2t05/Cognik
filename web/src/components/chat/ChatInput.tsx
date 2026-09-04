/**
 * ChatInput — 居中输入栏。流式生成时不禁用输入（支持 type-ahead 队列）。
 * streaming 时发送按钮变为停止按钮；队列中有消息时显示队列指示。
 */
'use client';

import { forwardRef } from 'react';
import { IconButton } from '@/components/ui/icon-button';
import { Textarea } from '@/components/ui/textarea';
import { Send, Square, Loader2, ListPlus } from 'lucide-react';

interface ChatInputProps {
  value: string;
  onChange: (v: string) => void;
  onSend: () => void;
  onStop?: () => void;
  disabled: boolean;
  loading: boolean;
  streaming: boolean;
  queueCount?: number;
  placeholder: string;
}

export const ChatInput = forwardRef<HTMLTextAreaElement, ChatInputProps>(
  ({ value, onChange, onSend, onStop, disabled, loading, streaming, queueCount = 0, placeholder }, ref) => {
    const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      // Alt+Enter / Shift+Enter 换行；Enter 发送（streaming 时进队列）
      if (e.key === 'Enter') {
        if (e.altKey || e.shiftKey) {
          // 允许默认换行，不拦截
          return;
        }
        e.preventDefault();
        if (value.trim()) onSend();
      }
    };

    return (
      <div className="border-t border-[var(--color-hairline)] bg-[var(--color-canvas)] px-4 py-3">
        <div className="max-w-[768px] mx-auto flex items-end gap-2">
          <div className="flex-1 relative">
            <Textarea
              ref={ref}
              value={value}
              onChange={(e) => onChange(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={streaming ? '输入消息加入队列…' : placeholder}
              disabled={disabled}
              aria-label="输入消息"
              rows={1}
              className="min-h-11 max-h-40 field-sizing-content py-2.5 pl-5 pr-20 text-body rounded-[var(--radius-lg)] border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] resize-none"
            />
            <span className="absolute right-4 top-3 text-fine text-[var(--color-text-muted-48)] pointer-events-none select-none">
              ⏎ 发送 · ⇧⏎ 换行
            </span>
          </div>
          {/* 队列指示 */}
          {queueCount > 0 && (
            <span className="flex items-center gap-1 text-[12px] text-[var(--color-text-muted-48)] px-2">
              <ListPlus size={14} />
              {queueCount}
            </span>
          )}
          {streaming ? (
            <IconButton label="停止生成" danger onClick={onStop}><Square /></IconButton>
          ) : (
            <IconButton label="发送" disabled={!value.trim() || disabled || loading} onClick={onSend}>{loading ? <Loader2 className="animate-spin" /> : <Send />}</IconButton>
          )}
        </div>
      </div>
    );
  }
);

ChatInput.displayName = 'ChatInput';
