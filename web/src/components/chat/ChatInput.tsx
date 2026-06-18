'use client';

import { forwardRef } from 'react';
import { AppleButton } from '@/components/ui/AppleButton';
import styles from './ChatInput.module.css';

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
      <div className={styles.wrapper}>
        <input
          ref={ref}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={disabled}
          className={styles.input}
        />
        <AppleButton onClick={onSend} loading={loading} disabled={!value.trim() || disabled}>
          发送
        </AppleButton>
      </div>
    );
  }
);

ChatInput.displayName = 'ChatInput';
