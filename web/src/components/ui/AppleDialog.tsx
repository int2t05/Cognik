/** AppleDialog — Radix Dialog 封装，Apple 样式 */
'use client';

import * as Dialog from '@radix-ui/react-dialog';
import { type ReactNode } from 'react';
import styles from './AppleDialog.module.css';

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
        <Dialog.Overlay className={styles.overlay} />
        <Dialog.Content
          className={styles.dialog}
          style={{ '--dialog-width': width } as React.CSSProperties}
        >
          <Dialog.Title className={styles.header}>{title}</Dialog.Title>
          {description && (
            <Dialog.Description className={styles.description}>
              {description}
            </Dialog.Description>
          )}
          <div className={styles.body}>{children}</div>
          {footer && <div className={styles.footer}>{footer}</div>}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
