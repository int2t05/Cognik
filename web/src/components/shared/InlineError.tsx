/** InlineError — 统一错误提示，支持内联与全页两种模式，消除各页面加载失败写法差异。 */
import { AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/button';

interface InlineErrorProps {
  message?: string;
  onRetry?: () => void;
  fullPage?: boolean;
}

export function InlineError({ message = '加载失败，请刷新重试', onRetry, fullPage = false }: InlineErrorProps) {
  if (fullPage) {
    return (
      <div className="flex flex-col items-center gap-2 py-10 text-caption text-[var(--color-error)]">
        <AlertTriangle size={20} />
        <span>{message}</span>
        {onRetry && (
          <Button variant="link" size="sm" onClick={onRetry} className="text-[var(--color-error)] underline">
            重试
          </Button>
        )}
      </div>
    );
  }

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
