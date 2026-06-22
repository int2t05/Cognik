/**
 * ChatInput — 豆包风格居中圆角药丸输入框。
 */
'use client';

import { forwardRef } from 'react';
import { AppleButton } from '@/components/ui/AppleButton';
import { Send } from 'lucide-react';

interface ChatInputProps {
  value: string;
  onChange: (v: string) => void;
  onSend: () => void;
  disabled: boolean;
  loading: boolean;
  placeholder: string;
}

export const ChatInput = forwardRef<HTMLInputElement, ChatInputProps>(
  ({ value, onChange, onSend, disabled, loading, placeholder }, ref) => {
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
              className="w-full h-11 px-5 text-body rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-parchment)] text-[var(--color-ink)] outline-none transition disabled:opacity-50 focus:border-[var(--color-accent)] focus:bg-[var(--color-canvas)]"
            />
          </div>
          <AppleButton
            onClick={onSend}
            loading={loading}
            disabled={!value.trim() || disabled}
            className="p-2 rounded-full"
            aria-label="发送"
          >
            <Send size={17} />
          </AppleButton>
        </div>
      </div>
    );
  }
);

ChatInput.displayName = 'ChatInput';
