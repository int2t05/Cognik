/**
 * AppleButton — 四种变体按钮。
 *
 * 严格对照 docs/prompts/ui.md 按钮体系：
 * - pill: 主 CTA，蓝色胶囊 (Action Blue #0066cc)
 * - ghost: 次要，蓝色边框+透明背景
 * - utility: 工具按钮，深色小圆角
 * - pearl: 珍珠胶囊，卡片内次要操作
 *
 * Active 态：transform: scale(0.95)（系统全局微交互）
 * Focus 态：outline: 2px solid var(--accent-focus)
 * 无阴影、无渐变。
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
