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
      <div style={{ display: 'flex', gap: 12 }}>
        <input
          ref={ref}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={disabled}
          style={{
            flex: 1, height: 44, padding: '0 20px', fontSize: 17,
            borderRadius: 'var(--radius-pill)', border: '1px solid var(--hairline)',
            background: 'var(--bg-canvas)', color: 'var(--text-ink)', outline: 'none',
            opacity: disabled ? 0.5 : 1,
          }}
        />
        <AppleButton onClick={onSend} loading={loading} disabled={!value.trim() || disabled}>
          发送
        </AppleButton>
      </div>
    );
  }
);

ChatInput.displayName = 'ChatInput';
