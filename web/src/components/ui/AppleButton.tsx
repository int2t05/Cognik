/**
 * AppleButton — 四种变体按钮（pill/ghost/utility/pearl），紧凑左图标右文字。
 */

import { type ButtonHTMLAttributes, forwardRef } from 'react';

type ButtonVariant = 'pill' | 'ghost' | 'utility' | 'pearl';

interface AppleButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  loading?: boolean;
}

const variantClasses: Record<ButtonVariant, string> = {
  pill: 'bg-[var(--color-accent)] text-[var(--color-on-accent)] text-fine rounded-[var(--radius-pill)] px-4 py-1.5',
  ghost: 'bg-transparent text-[var(--color-accent)] text-fine rounded-[var(--radius-pill)] px-2.5 py-1',
  utility: 'bg-[var(--color-pearl)] text-[var(--color-text-muted-80)] text-fine rounded-[var(--radius-pill)] px-2.5 py-1 border border-[var(--color-divider-soft)]',
  pearl: 'bg-[var(--color-pearl)] text-[var(--color-text-muted-80)] text-fine rounded-[var(--radius-pill)] px-2.5 py-1',
};

export const AppleButton = forwardRef<HTMLButtonElement, AppleButtonProps>(
  ({ variant = 'pill', loading = false, disabled, className = '', children, ...rest }, ref) => {
    const classes = [
      'inline-flex items-center gap-1 font-medium cursor-pointer border-0 font-sans whitespace-nowrap select-none transition-all duration-150 active:scale-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent-focus)] disabled:opacity-40 disabled:cursor-not-allowed',
      variantClasses[variant],
      className,
    ].filter(Boolean).join(' ');

    return (
      <button ref={ref} type="button" className={classes} disabled={disabled || loading} {...rest}>
        {loading && (
          <span className="inline-block w-3.5 h-3.5 border-2 border-current border-t-transparent rounded-full animate-spin shrink-0" aria-hidden="true" />
        )}
        <span className={loading ? 'opacity-70' : ''}>{children}</span>
      </button>
    );
  },
);

AppleButton.displayName = 'AppleButton';
