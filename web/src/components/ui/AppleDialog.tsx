/** AppleDialog — Radix Dialog 封装，Apple 风格动画。 */
'use client';

import * as Dialog from '@radix-ui/react-dialog';
import { type ReactNode } from 'react';

interface AppleDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string;
  width?: string;
  children: ReactNode;
  footer?: ReactNode;
}

export function AppleDialog({
  open,
  onOpenChange,
  title,
  description,
  width = '480px',
  children,
  footer,
}: AppleDialogProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        {/* 遮罩：淡入/淡出 */}
        <Dialog.Overlay
          className="fixed inset-0 z-[var(--z-dialog)]"
          style={{
            backgroundColor: 'var(--color-overlay)',
            animation: `${open ? 'overlay-in' : 'overlay-out'} 200ms cubic-bezier(0.16, 1, 0.3, 1) forwards`,
          }}
        />
        {/* 内容：缩放+淡入组合 */}
        <Dialog.Content
          className="fixed left-1/2 top-1/2 z-[calc(var(--z-dialog)+1)] max-h-[85vh] max-w-[90vw] overflow-y-auto rounded-[var(--radius-lg)] bg-[var(--color-canvas)] shadow-[var(--shadow-dialog)]"
          style={{
            width,
            animation: `${open ? 'dialog-in' : 'dialog-out'} 250ms cubic-bezier(0.16, 1, 0.3, 1) forwards`,
          } as React.CSSProperties}
        >
          <Dialog.Title className="px-6 pt-5 pb-2 text-title font-semibold text-[var(--color-ink)]">
            {title}
          </Dialog.Title>
          {description && (
            <Dialog.Description className="text-caption text-[var(--color-text-muted-48)] mt-1 px-6">
              {description}
            </Dialog.Description>
          )}
          <div className="px-6 py-5">{children}</div>
          {footer && (
            <div className="px-6 py-4 flex gap-2 justify-end border-t border-[var(--color-divider-soft)]">
              {footer}
            </div>
          )}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
