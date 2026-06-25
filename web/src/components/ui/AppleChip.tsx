/**
 * AppleChip — 切换芯片按钮，对齐 Apple HIG segmented-controls / configurator-option-chip。
 *
 * 统一角色选择、权限选择、日期预设、筛选按钮组等 toggle chip 模式。
 * 提供 sm/md 两种尺寸，支持可选 icon，键盘可访问。
 *
 * 设计依据：Apple HIG — pill 圆角、Action Blue 单色、44px 触摸目标（md）、
 * active:scale-95 微交互、focus-visible 蓝色焦点环。
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

interface AppleChipProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** 是否选中 */
  selected: boolean;
  /** 尺寸：sm=表单内密集芯片 | md=筛选栏/预设按钮 */
  size?: 'sm' | 'md';
  /** 可选左侧图标 */
  icon?: ReactNode;
}

const sizeMeta: Record<'sm' | 'md', { base: string; iconSize: number }> = {
  sm: {
    // 对齐 AppleButton ghost 的 padding/字号，8px 网格 + 44px 最小触摸目标
    base: 'text-caption py-2 px-4 gap-1.5',
    iconSize: 14,
  },
  md: {
    // 对齐 AppleButton pill 的 padding/字号，12×24px 内边距
    base: 'text-body py-3 px-6 gap-2',
    iconSize: 16,
  },
};

export const AppleChip = forwardRef<HTMLButtonElement, AppleChipProps>(
  ({ selected, size = 'sm', icon, className = '', children, ...rest }, ref) => {
    const meta = sizeMeta[size];
    const hasChildren = Children.count(children) > 0;

    const sizedIcon =
      icon && isValidElement(icon)
        ? cloneElement(icon as ReactElement<{ size?: number }>, { size: meta.iconSize })
        : icon;

    const classes = [
      'inline-flex items-center justify-center font-sans font-normal whitespace-nowrap select-none',
      'rounded-[var(--radius-pill)] border cursor-pointer transition-all duration-150',
      'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent-focus)]',
      'active:scale-95',
      meta.base,
      selected
        ? 'bg-[var(--color-accent)] border-[var(--color-accent)] text-[var(--color-on-accent)]'
        : 'bg-transparent border-[var(--color-hairline)] text-[var(--color-ink)] hover:bg-[var(--color-divider-soft)]',
      className,
    ].filter(Boolean).join(' ');

    return (
      <button ref={ref} type="button" className={classes} {...rest}>
        {sizedIcon}
        {hasChildren && <span>{children}</span>}
      </button>
    );
  },
);

AppleChip.displayName = 'AppleChip';
