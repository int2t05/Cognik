/**
 * ChatInput — 豆包风格居中药丸输入框，流式中显示 Stop 按钮（AI Chat 最佳实践）。
 */
'use client';

import { forwardRef } from 'react';
import { AppleButton } from '@/components/ui/AppleButton';
import { Send, Square } from 'lucide-react';

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
            <input
              ref={ref}
              value={value}
              onChange={(e) => onChange(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={placeholder}
              disabled={disabled}
              aria-label="输入消息"
              className="w-full h-11 pr-24 pl-5 text-body rounded-[var(--radius-lg)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none transition disabled:opacity-40 disabled:cursor-not-allowed focus:border-[var(--color-accent)] focus:shadow-[var(--focus-ring)]"
            />
            <span className="absolute right-4 top-1/2 -translate-y-1/2 text-fine text-[var(--color-text-muted-48)] pointer-events-none select-none">
              Enter ↵
            </span>
          </div>
          {streaming ? (
            <button
              onClick={onStop}
              aria-label="停止生成"
              className="flex items-center justify-center w-11 h-11 rounded-[var(--radius-pill)] bg-[var(--color-error)] text-[var(--color-on-accent)] border-0 cursor-pointer transition hover:opacity-90 active:scale-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent-focus)]"
            >
              <Square size={16} fill="currentColor" />
            </button>
          ) : (
            <AppleButton
              icon={<Send />}
              onClick={onSend}
              loading={loading}
              disabled={!value.trim() || disabled}
              aria-label="发送"
            />
          )}
        </div>
      </div>
    );
  }
);

ChatInput.displayName = 'ChatInput';
