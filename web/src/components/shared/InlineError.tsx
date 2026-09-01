/** InlineError — 内联错误提示，统一各页面加载失败样式。 */
import { AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/button';

interface InlineErrorProps {
  message?: string;
  onRetry?: () => void;
}

export function InlineError({ message = '加载失败，请刷新重试', onRetry }: InlineErrorProps) {
  return (
    <div className="flex items-center gap-2 text-caption text-[var(--color-error)] mb-4">
      <AlertTriangle size={12} />
      <span>{message}</span>
      {onRetry && (
        <Button variant="link" size="sm" onClick={onRetry} className="text-[var(--color-error)] underline">
          重试
        </Button>
      )}
    </div>
  );
}
