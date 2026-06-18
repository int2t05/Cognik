/**
 * AppleButton — 四种变体按钮（pill/ghost/utility/pearl）。
 * 对照 docs/prompts/ui.md 按钮体系。
 */

import { type ButtonHTMLAttributes, forwardRef } from 'react';
import styles from './AppleButton.module.css';

type ButtonVariant = 'pill' | 'ghost' | 'utility' | 'pearl';

interface AppleButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  loading?: boolean;
}

export const AppleButton = forwardRef<HTMLButtonElement, AppleButtonProps>(
  ({ variant = 'pill', loading = false, disabled, className = '', children, ...rest }, ref) => {
    const classes = [styles.btn, styles[variant], className].filter(Boolean).join(' ');

    return (
      <button
        ref={ref}
        type="button"
        className={classes}
        disabled={disabled || loading}
        {...rest}
      >
        {loading && <span className={styles.spinner} aria-hidden="true" />}
        <span className={loading ? styles.loadingLabel : ''}>{children}</span>
      </button>
    );
  }
);

AppleButton.displayName = 'AppleButton';
