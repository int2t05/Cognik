/** AppleDialog — Radix Dialog 封装，Apple 样式 */
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
        <Dialog.Overlay className="fixed inset-0 bg-black/40 flex items-center justify-center z-[1000] backdrop-blur-sm" />
        <Dialog.Content
          className="fixed left-1/2 top-1/2 z-[1001] max-h-[85vh] max-w-[90vw] -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-[var(--radius-lg)] bg-[var(--color-canvas)] shadow-[0_20px_60px_rgba(0,0,0,0.15)]"
          style={{ width } as React.CSSProperties}
        >
          <Dialog.Title className="px-6 pt-5 pb-0 text-[17px] font-medium text-[var(--color-ink)]">
            {title}
          </Dialog.Title>
          {description && (
            <Dialog.Description className="text-sm text-[var(--color-text-muted-48)] mt-1 px-6">
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
