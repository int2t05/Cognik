/**
 * AppleButton — 五变体按钮，icon 与文字协调排列。
 *
 * 设计依据：Apple HIG button-primary（17px/400/11×22px）、
 * button-pearl-capsule（14px/8×14px）、touch-target 44×44px。
 *
 * 层级：pill（主要 CTA）> pillOutline（次要 CTA）> ghost / utility（紧凑操作）
 * 新增 danger 变体用于不可逆操作，icon 属性让图标与文字自动协调。
 */

import {
  type ButtonHTMLAttributes,
  type ReactElement,
  type ReactNode,
  Children,
  cloneElement,
  forwardRef,
  isValidElement,
} from 'react';

type ButtonVariant = 'pill' | 'pillOutline' | 'ghost' | 'utility' | 'danger';

interface AppleButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** 变体：pill=首要CTA | pillOutline=次要CTA | ghost=极简 | utility=珍珠底 | danger=危险操作 */
  variant?: ButtonVariant;
  /** 左侧图标，传入 Lucide 组件即可（无需手动设 size） */
  icon?: ReactNode;
  loading?: boolean;
  spanClassName?: string;
}

// ── 变体样式映射 ──────────────────────────────────────────────
// 大按钮（pill / pillOutline / danger）：17px body / 11×22px padding / 18px icon
// 小按钮（ghost / utility）：14px caption / 7×15px padding / 16px icon

const variantMeta: Record<ButtonVariant, {
  base: string;
  iconSize: number;
}> = {
  pill: {
    base: 'bg-[var(--color-accent)] text-[var(--color-on-accent)] rounded-[var(--radius-pill)] text-body py-[11px] px-[22px]',
    iconSize: 18,
  },
  pillOutline: {
    base: 'bg-transparent text-[var(--color-accent)] rounded-[var(--radius-pill)] text-body py-[11px] px-[22px] border border-[var(--color-accent)]',
    iconSize: 18,
  },
  danger: {
    base: 'bg-[var(--color-error)] text-white rounded-[var(--radius-pill)] text-body py-[11px] px-[22px]',
    iconSize: 18,
  },
  ghost: {
    base: 'bg-transparent text-[var(--color-accent)] rounded-[var(--radius-pill)] text-caption py-[7px] px-[15px]',
    iconSize: 16,
  },
  utility: {
    base: 'bg-[var(--color-pearl)] text-[var(--color-text-muted-80)] rounded-[var(--radius-pill)] text-caption py-[7px] px-[15px] border border-[var(--color-divider-soft)]',
    iconSize: 16,
  },
};

export const AppleButton = forwardRef<HTMLButtonElement, AppleButtonProps>(
  ({ variant = 'pill', icon, loading = false, disabled, className = '', children, spanClassName, ...rest }, ref) => {
    const meta = variantMeta[variant];
    const hasChildren = Children.count(children) > 0;
    const isIconOnly = icon && !hasChildren;

    // 自动为 icon 注入尺寸，保证与文字协调
    const sizedIcon =
      icon && isValidElement(icon)
        ? cloneElement(icon as ReactElement<{ size?: number }>, { size: meta.iconSize })
        : icon;

    const classes = [
      // 公共基础：flex 居中、无衬线、不可选中、过渡动画、微交互、焦点环、禁用态
      'inline-flex items-center justify-center font-sans font-normal whitespace-nowrap select-none',
      'transition-all duration-150 active:scale-95',
      'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent-focus)]',
      'disabled:opacity-40 disabled:cursor-not-allowed disabled:active:scale-100',
      // 按文本/纯图标自动适配间距
      hasChildren ? 'gap-2' : '',
      // 纯图标模式：44×44px 最小触摸目标
      isIconOnly ? 'p-[13px]' : '',
      meta.base,
      className,
    ].filter(Boolean).join(' ');

    return (
      <button ref={ref} type="button" className={classes} disabled={disabled || loading} {...rest}>
        {/* loading 态：spinner 尺寸匹配 icon 尺寸 */}
        {loading && (
          <span
            className="inline-block shrink-0 border-2 border-current border-t-transparent rounded-full animate-spin"
            style={{ width: meta.iconSize, height: meta.iconSize }}
            aria-hidden="true"
          />
        )}
        {/* 非 loading 态：显示 icon（如有） */}
        {!loading && sizedIcon}
        {/* 文字内容：loading 时降低透明度给出视觉反馈 */}
        {hasChildren && <span className={loading ? 'opacity-70' : '' + spanClassName}>{children}</span>}
      </button>
    );
  },
);

AppleButton.displayName = 'AppleButton';
