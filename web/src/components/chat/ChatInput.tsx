/**
 * ChatInput — 居中输入栏。流式生成时不禁用输入（支持 type-ahead 队列）。
 * streaming 时发送按钮变为停止按钮；队列中有消息时显示队列指示。
 */
'use client';

import { forwardRef } from 'react';
import { IconButton } from '@/components/ui/icon-button';
import { Input } from '@/components/ui/input';
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

export const ChatInput = forwardRef<HTMLInputElement, ChatInputProps>(
  ({ value, onChange, onSend, onStop, disabled, loading, streaming, queueCount = 0, placeholder }, ref) => {
    const handleKeyDown = (e: React.KeyboardEvent) => {
      // streaming 时也允许 Enter 发送（进队列）
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        if (value.trim()) onSend();
      }
    };

    return (
      <div className="border-t border-[var(--color-hairline)] bg-[var(--color-canvas)] px-4 py-3">
        <div className="max-w-[768px] mx-auto flex items-center gap-2">
          <div className="flex-1 relative">
            <Input
              ref={ref}
              value={value}
              onChange={(e) => onChange(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={streaming ? '输入消息加入队列…' : placeholder}
              disabled={disabled}
              aria-label="输入消息"
              className="h-11 pr-20 pl-5 text-body rounded-[var(--radius-lg)] border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)]"
            />
            <span className="absolute right-4 top-1/2 -translate-y-1/2 text-fine text-[var(--color-text-muted-48)] pointer-events-none select-none">
              Enter ↵
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
