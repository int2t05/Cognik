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

export function AppleDialog({ open, onOpenChange, title, description, width = '480px', children, footer }: AppleDialogProps) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.4)',
            backdropFilter: 'blur(4px)',
            zIndex: 1000,
          }}
        />
        <Dialog.Content
          style={{
            position: 'fixed',
            top: '50%',
            left: '50%',
            transform: 'translate(-50%, -50%)',
            width,
            maxWidth: 'calc(100vw - 32px)',
            maxHeight: 'calc(100vh - 64px)',
            overflow: 'auto',
            background: 'var(--bg-canvas)',
            borderRadius: 'var(--radius-lg)',
            padding: 'var(--space-xxl)',
            zIndex: 1001,
            boxShadow: '0 25px 50px rgba(0,0,0,0.25)',
          }}
        >
          <Dialog.Title style={{ fontSize: 20, fontWeight: 600, color: 'var(--text-ink)', marginBottom: 4 }}>
            {title}
          </Dialog.Title>
          {description && (
            <Dialog.Description style={{ fontSize: 14, color: 'var(--text-muted-48)', marginBottom: 24 }}>
              {description}
            </Dialog.Description>
          )}
          <div style={{ marginBottom: footer ? 24 : 0 }}>{children}</div>
          {footer && <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 12 }}>{footer}</div>}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
