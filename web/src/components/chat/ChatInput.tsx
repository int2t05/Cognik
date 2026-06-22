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
      <div className="flex gap-3 border-t border-[var(--color-hairline)] px-4 lg:px-6 py-4 bg-[var(--color-canvas)]">
        <input
          ref={ref}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={disabled}
	          aria-label="输入消息"
          className="flex-1 h-12 px-5 text-body rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none transition disabled:opacity-50 focus:border-[var(--color-accent)]"
        />
        <AppleButton onClick={onSend} loading={loading} disabled={!value.trim() || disabled} className="p-2" aria-label="发送">
          <Send size={17} />
        </AppleButton>
      </div>
    );
  }
);

ChatInput.displayName = 'ChatInput';
