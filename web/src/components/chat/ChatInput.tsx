/**
 * ChatInput — 居中输入栏。流式生成时不禁用输入（支持 type-ahead 队列）。
 * streaming 时发送按钮变为停止按钮；队列中有消息时显示队列指示。
 */
'use client';

import { forwardRef } from 'react';
import { useTranslations } from 'next-intl';
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
    const t = useTranslations();
    const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      // Alt+Enter / Shift+Enter / Ctrl+Enter 换行；Enter 发送（streaming 时进队列）
      if (e.key === 'Enter') {
        if (e.altKey || e.shiftKey || e.ctrlKey) {
          // 手动在光标处插入换行，避免部分浏览器/IME 拦截 Alt+Enter 默认行为
          e.preventDefault();
          const ta = e.currentTarget;
          const start = ta.selectionStart;
          const end = ta.selectionEnd;
          const next = value.slice(0, start) + '\n' + value.slice(end);
          onChange(next);
          // 恢复光标到换行后
          requestAnimationFrame(() => { ta.selectionStart = ta.selectionEnd = start + 1; });
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
              placeholder={streaming ? t('chat.queueInputPlaceholder') : placeholder}
              disabled={disabled}
              aria-label={t('chat.inputAria')}
              rows={1}
              className="min-h-11 max-h-40 field-sizing-content py-2.5 pl-5 pr-20 text-body rounded-[var(--radius-lg)] border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] resize-none"
            />
            <span className="absolute right-4 top-3 text-fine text-[var(--color-text-muted-48)] pointer-events-none select-none">
              {t('chat.sendHint')}
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
            <IconButton label={t('chat.stop')} danger onClick={onStop}><Square /></IconButton>
          ) : (
            <IconButton label={t('chat.send')} disabled={!value.trim() || disabled || loading} onClick={onSend}>{loading ? <Loader2 className="animate-spin" /> : <Send />}</IconButton>
          )}
        </div>
      </div>
    );
  }
);

ChatInput.displayName = 'ChatInput';
