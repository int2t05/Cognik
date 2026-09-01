/**
 * ChatInput — 居中输入栏。流式生成时发送按钮原位变为停止按钮。
 */
'use client';

import { forwardRef } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Send, Square, Loader2 } from 'lucide-react';

interface ChatInputProps {
  value: string;
  onChange: (v: string) => void;
  onSend: () => void;
  onStop?: () => void;
  disabled: boolean;
  loading: boolean;
  streaming: boolean;
  placeholder: string;
}

export const ChatInput = forwardRef<HTMLInputElement, ChatInputProps>(
  ({ value, onChange, onSend, onStop, disabled, loading, streaming, placeholder }, ref) => {
    const handleKeyDown = (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); onSend(); }
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
              placeholder={placeholder}
              disabled={disabled}
              aria-label="输入消息"
              className="h-11 pr-20 pl-5 text-body rounded-[var(--radius-lg)] border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)]"
            />
            <span className="absolute right-4 top-1/2 -translate-y-1/2 text-fine text-[var(--color-text-muted-48)] pointer-events-none select-none">
              Enter ↵
            </span>
          </div>
          {streaming ? (
            <Button variant="destructive" size="icon" onClick={onStop} aria-label="停止生成"><Square /></Button>
          ) : (
            <Button size="icon" disabled={!value.trim() || disabled || loading} onClick={onSend} aria-label="发送">{loading ? <Loader2 className="animate-spin" /> : <Send />}</Button>
          )}
        </div>
      </div>
    );
  }
);

ChatInput.displayName = 'ChatInput';
