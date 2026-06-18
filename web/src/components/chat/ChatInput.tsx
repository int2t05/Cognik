'use client';

import { forwardRef } from 'react';
import { AppleButton } from '@/components/ui/AppleButton';

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
      <div className="flex gap-3">
        <input
          ref={ref}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={disabled}
          className="flex-1 h-11 px-5 text-[15px] rounded-[var(--radius-pill)] border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none transition disabled:opacity-50 focus:border-[var(--color-accent)]"
        />
        <AppleButton onClick={onSend} loading={loading} disabled={!value.trim() || disabled}>
          发送
        </AppleButton>
      </div>
    );
  }
);

ChatInput.displayName = 'ChatInput';
