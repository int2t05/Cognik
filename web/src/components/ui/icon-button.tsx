'use client';

import { forwardRef, type ReactNode } from 'react';
import { Tooltip } from 'radix-ui';
import { cn } from '@/lib/utils';

type ButtonVariant = 'default' | 'ghost' | 'destructive' | 'outline' | 'secondary' | 'link' | 'menu';
type ButtonSize = 'sm' | 'default' | 'lg' | 'icon' | 'icon-sm';

/** IconButton — 全站统一按钮组件，取代散落的 Button。
 *  所有 variant 默认透明背景，hover 时按语义变色（品牌色/红色/静音瓦片）。
 *  label：纯图标按钮必填（作 aria-label + tooltip）；带文字按钮留空（文字即标签）。
 *  支持任意 children：纯图标、纯文字、图标+文字。 */
const IconButtonImpl = forwardRef<
  HTMLButtonElement,
  Omit<React.ComponentProps<'button'>, 'children'> & {
    children: ReactNode;
    /** 纯图标按钮的用途提示（作 aria-label + tooltip）；带文字按钮留空 */
    label?: string;
    /** 视觉样式，默认 ghost（透明+hover 品牌色） */
    variant?: ButtonVariant;
    /** 尺寸：sm/default/lg 文字按钮；icon/icon-sm 图标按钮 */
    size?: ButtonSize;
    /** 破坏性操作（删除等）的快捷标记，等价 variant="destructive" */
    danger?: boolean;
  }
>(function IconButton({ className, children, label, variant = 'ghost', size = 'default', danger, ...props }, ref) {
  const btn = (
    <button
      ref={ref}
      type="button"
      aria-label={label}
      data-slot="button"
      data-variant={danger ? 'destructive' : variant}
      data-size={size}
      className={cn(
        'inline-flex shrink-0 items-center justify-center gap-2 rounded-[var(--radius-md)] text-sm font-medium whitespace-nowrap outline-none transition-colors',
        'active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50',
        'focus-visible:ring-[3px] focus-visible:ring-[var(--color-accent)]/30',
        '[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*=size-])]:size-4',
        // 所有 variant 透明背景，hover 按语义变色
        variant === 'default' && 'bg-transparent text-[var(--color-ink)] hover:bg-[var(--color-accent)]/10 hover:text-[var(--color-accent)]',
        variant === 'ghost' && 'bg-transparent text-[var(--color-text-muted-80)] hover:bg-[var(--color-accent)]/10 hover:text-[var(--color-accent)]',
        variant === 'destructive' && 'bg-transparent text-[var(--color-ink)] hover:bg-[var(--color-error)]/10 hover:text-[var(--color-error)]',
        variant === 'outline' && 'border border-[var(--color-hairline)] bg-transparent text-[var(--color-ink)] hover:bg-[var(--color-accent)]/10 hover:text-[var(--color-accent)] hover:border-[var(--color-accent)]/40',
        variant === 'secondary' && 'bg-transparent text-[var(--color-text-muted-80)] hover:bg-[var(--color-tile-1)] hover:text-[var(--color-ink)]',
        variant === 'link' && 'bg-transparent text-[var(--color-accent)] underline-offset-4 hover:underline',
        variant === 'menu' && 'bg-transparent text-[var(--color-ink)] hover:bg-[var(--color-divider-soft)] text-callout font-medium',
        danger && 'bg-transparent text-[var(--color-ink)] hover:bg-[var(--color-error)]/10 hover:text-[var(--color-error)]',
        // 尺寸
        size === 'sm' && 'h-8 gap-1.5 rounded-md px-3 has-[>svg]:px-2.5',
        size === 'default' && 'h-9 px-4 py-2 has-[>svg]:px-3',
        size === 'lg' && 'h-10 rounded-md px-6 has-[>svg]:px-4',
        size === 'icon' && 'size-9',
        size === 'icon-sm' && 'size-8',
        className,
      )}
      {...props}
    >
      {children}
    </button>
  );

  // 带文字按钮无 label → 不包 tooltip（文字即标签）；纯图标按钮 label → 包 tooltip
  if (!label) return btn;

  return (
    <Tooltip.Root>
      <Tooltip.Trigger asChild>{btn}</Tooltip.Trigger>
      <Tooltip.Portal>
        <Tooltip.Content
          side="bottom"
          sideOffset={6}
          className={cn(
            'z-[var(--z-overlay)] rounded-[var(--radius-sm)] px-2 py-1 text-fine text-white',
            'bg-[var(--color-ink)] shadow-md',
            'data-[state=delayed-open]:animate-in data-[state=delayed-open]:fade-in-0 data-[state=delayed-open]:zoom-in-95',
            'data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95',
          )}
        >
          {label}
          <Tooltip.Arrow className="fill-[var(--color-ink)]" />
        </Tooltip.Content>
      </Tooltip.Portal>
    </Tooltip.Root>
  );
});

export { IconButtonImpl as IconButton };
