/** ConfirmDialog — 危险操作二次确认。基于 shadcn Dialog compound，外部 API 不变。 */
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { AppleButton } from '@/components/ui/AppleButton';

interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  message?: string;
  confirmLabel?: string;
  onConfirm: () => void;
  loading?: boolean;
  danger?: boolean;
  children?: React.ReactNode;
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  message,
  confirmLabel = '确认',
  onConfirm,
  loading,
  danger,
  children,
}: ConfirmDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {message && <DialogDescription>{message}</DialogDescription>}
        </DialogHeader>
        {children ?? <div />}
        <DialogFooter>
          <AppleButton variant="ghost" onClick={() => onOpenChange(false)}>
            取消
          </AppleButton>
          <AppleButton
            variant={danger ? 'danger' : 'pill'}
            onClick={onConfirm}
            loading={loading}
          >
            {confirmLabel}
          </AppleButton>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
