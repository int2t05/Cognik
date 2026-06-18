/** AppleInput — pill 圆角搜索框 + 标准输入 + textarea */
'use client';

import { type InputHTMLAttributes, type TextareaHTMLAttributes, forwardRef } from 'react';
import styles from './AppleInput.module.css';

interface AppleInputProps extends InputHTMLAttributes<HTMLInputElement> {
  pill?: boolean;
  label?: string;
  error?: string;
}

export const AppleInput = forwardRef<HTMLInputElement, AppleInputProps>(
  ({ pill, label, error, className = '', ...rest }, ref) => {
    const inputClass = [styles.input, pill ? styles.pill : '', error ? styles.inputError : '', className]
      .filter(Boolean).join(' ');

    return (
      <div className={label || error ? styles.group : ''}>
        {label && <label className={styles.label}>{label}</label>}
        <input ref={ref} className={inputClass} {...rest} />
        {error && <p className={styles.errorText}>{error}</p>}
      </div>
    );
  }
);
AppleInput.displayName = 'AppleInput';

// AppleTextarea — textarea 变体
interface AppleTextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
  error?: string;
}

export const AppleTextarea = forwardRef<HTMLTextAreaElement, AppleTextareaProps>(
  ({ label, error, rows = 4, className = '', ...rest }, ref) => {
    const textareaClass = [styles.textarea, error ? styles.textareaError : '', className]
      .filter(Boolean).join(' ');

    return (
      <div className={styles.group}>
        {label && <label className={styles.label}>{label}</label>}
        <textarea ref={ref} rows={rows} className={textareaClass} {...rest} />
        {error && <p className={styles.errorText}>{error}</p>}
      </div>
    );
  }
);
AppleTextarea.displayName = 'AppleTextarea';
