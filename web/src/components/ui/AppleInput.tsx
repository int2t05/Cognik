/** AppleInput — pill 圆角搜索框 + 标准输入 + textarea */
'use client';

import { type InputHTMLAttributes, type TextareaHTMLAttributes, forwardRef } from 'react';

interface AppleInputProps extends InputHTMLAttributes<HTMLInputElement> {
  pill?: boolean;
  label?: string;
  error?: string;
}

export const AppleInput = forwardRef<HTMLInputElement, AppleInputProps>(
  ({ pill, label, error, className = '', ...rest }, ref) => (
    <div style={{ marginBottom: label || error ? 16 : 0 }}>
      {label && (
        <label style={{ display: 'block', fontSize: 14, fontWeight: 500, marginBottom: 6, color: 'var(--text-ink)' }}>
          {label}
        </label>
      )}
      <input
        ref={ref}
        style={{
          width: '100%',
          height: 44,
          padding: '0 16px',
          fontSize: 17,
          borderRadius: pill ? 'var(--radius-pill)' : 'var(--radius-sm)',
          border: error ? '1px solid var(--color-error)' : '1px solid var(--hairline)',
          background: 'var(--bg-canvas)',
          color: 'var(--text-ink)',
          outline: 'none',
          boxSizing: 'border-box',
        }}
        {...rest}
      />
      {error && <p style={{ fontSize: 12, color: 'var(--color-error)', marginTop: 4 }}>{error}</p>}
    </div>
  )
);
AppleInput.displayName = 'AppleInput';

// AppleTextarea — textarea 变体
interface AppleTextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
  error?: string;
}

export const AppleTextarea = forwardRef<HTMLTextAreaElement, AppleTextareaProps>(
  ({ label, error, rows = 4, ...rest }, ref) => (
    <div style={{ marginBottom: 16 }}>
      {label && (
        <label style={{ display: 'block', fontSize: 14, fontWeight: 500, marginBottom: 6, color: 'var(--text-ink)' }}>
          {label}
        </label>
      )}
      <textarea
        ref={ref}
        rows={rows}
        style={{
          width: '100%',
          padding: '12px 16px',
          fontSize: 17,
          lineHeight: 1.47,
          borderRadius: 'var(--radius-sm)',
          border: error ? '1px solid var(--color-error)' : '1px solid var(--hairline)',
          background: 'var(--bg-canvas)',
          color: 'var(--text-ink)',
          outline: 'none',
          boxSizing: 'border-box',
          resize: 'vertical',
          fontFamily: 'var(--font-body)',
        }}
        {...rest}
      />
      {error && <p style={{ fontSize: 12, color: 'var(--color-error)', marginTop: 4 }}>{error}</p>}
    </div>
  )
);
AppleTextarea.displayName = 'AppleTextarea';
