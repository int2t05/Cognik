/** InlineError — 内联错误提示，统一各页面加载失败样式。 */
import { AlertTriangle } from 'lucide-react';

interface InlineErrorProps {
  message?: string;
  onRetry?: () => void;
}

export function InlineError({ message = '加载失败，请刷新重试', onRetry }: InlineErrorProps) {
  return (
    <div className="flex items-center gap-2 text-caption text-[var(--color-error)] mb-4">
      <AlertTriangle size={14} />
      <span>{message}</span>
      {onRetry && (
        <button
          onClick={onRetry}
          className="underline cursor-pointer border-0 bg-transparent text-[var(--color-error)] hover:opacity-70 transition"
        >
          重试
        </button>
      )}
    </div>
  );
}
