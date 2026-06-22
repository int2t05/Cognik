/** AppleInput — pill 圆角搜索框 + 标准输入 + textarea */
'use client';

import { useId, type InputHTMLAttributes, type TextareaHTMLAttributes, forwardRef } from 'react';

interface AppleInputProps extends InputHTMLAttributes<HTMLInputElement> {
  pill?: boolean;
  label?: string;
  error?: string;
}

export const AppleInput = forwardRef<HTMLInputElement, AppleInputProps>(
  ({ pill, label, error, className = '', id, ...rest }, ref) => {
    const generatedId = useId();
    const inputId = id || generatedId;
    const errorId = `${inputId}-error`;

    const inputClass = [
      'w-full h-11 px-4 text-body rounded-lg border bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none transition focus:border-[var(--color-accent)] focus:shadow-[var(--focus-ring)]',
      error ? 'border-[var(--color-error)]' : 'border-[var(--color-hairline)]',
      pill ? 'rounded-[var(--radius-pill)]' : '',
      className,
    ]
      .filter(Boolean)
      .join(' ');

    return (
      <div className={label || error ? 'mb-4' : ''}>
        {label && (
          <label htmlFor={inputId} className="block text-sm font-medium mb-1.5 text-[var(--color-ink)]">{label}</label>
        )}
        <input
          ref={ref}
          id={inputId}
          className={inputClass}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? errorId : undefined}
          {...rest}
        />
        {error && <p id={errorId} className="text-fine text-[var(--color-error)] mt-1" role="alert">{error}</p>}
      </div>
    );
  },
);
AppleInput.displayName = 'AppleInput';

// AppleTextarea — textarea 变体
interface AppleTextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
  error?: string;
}

export const AppleTextarea = forwardRef<HTMLTextAreaElement, AppleTextareaProps>(
  ({ label, error, rows = 4, className = '', id, ...rest }, ref) => {
    const generatedId = useId();
    const textareaId = id || generatedId;
    const errorId = `${textareaId}-error`;

    const textareaClass = [
      'w-full px-4 py-3 text-body leading-relaxed rounded-lg border bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none resize-y font-sans transition focus:border-[var(--color-accent)] focus:shadow-[var(--focus-ring)]',
      error ? 'border-[var(--color-error)]' : 'border-[var(--color-hairline)]',
      className,
    ]
      .filter(Boolean)
      .join(' ');

    return (
      <div className="mb-4">
        {label && (
          <label htmlFor={textareaId} className="block text-sm font-medium mb-1.5 text-[var(--color-ink)]">{label}</label>
        )}
        <textarea
          ref={ref}
          id={textareaId}
          rows={rows}
          className={textareaClass}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? errorId : undefined}
          {...rest}
        />
        {error && <p id={errorId} className="text-fine text-[var(--color-error)] mt-1" role="alert">{error}</p>}
      </div>
    );
  },
);
AppleTextarea.displayName = 'AppleTextarea';
