/** ConfirmDialog — 危险操作二次确认 */
import { AppleDialog } from '@/components/ui/AppleDialog';
import { AppleButton } from '@/components/ui/AppleButton';
import styles from './ConfirmDialog.module.css';

interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  message: string;
  confirmLabel?: string;
  onConfirm: () => void;
  loading?: boolean;
  danger?: boolean;
}

export function ConfirmDialog({ open, onOpenChange, title, message, confirmLabel = '确认', onConfirm, loading, danger }: ConfirmDialogProps) {
  return (
    <AppleDialog
      open={open}
      onOpenChange={onOpenChange}
      title={title}
      description={message}
      footer={
        <>
          <AppleButton variant="ghost" onClick={() => onOpenChange(false)}>取消</AppleButton>
          <AppleButton
            variant="pill"
            onClick={onConfirm}
            loading={loading}
            className={danger ? styles.dangerBtn : ''}
          >
            {confirmLabel}
          </AppleButton>
        </>
      }
    >
      <div />
    </AppleDialog>
  );
}
