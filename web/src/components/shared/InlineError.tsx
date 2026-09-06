/** InlineError — 统一错误提示，支持内联与全页两种模式，消除各页面加载失败写法差异。 */
'use client';

import { useTranslations } from 'next-intl';
import { AlertTriangle } from 'lucide-react';
import { IconButton } from '@/components/ui/icon-button';

interface InlineErrorProps {
  message?: string;
  onRetry?: () => void;
  fullPage?: boolean;
}

export function InlineError({ message, onRetry, fullPage = false }: InlineErrorProps) {
  const t = useTranslations();
  const msg = message ?? t('common.loadFailed');
  if (fullPage) {
    return (
      <div className="flex flex-col items-center gap-2 py-10 text-caption text-[var(--color-error)]">
        <AlertTriangle size={20} />
        <span>{msg}</span>
        {onRetry && (
          <IconButton variant="link" size="sm" onClick={onRetry} className="text-[var(--color-error)] underline">
            {t('common.retry')}
          </IconButton>
        )}
      </div>
    );
  }

  return (
    <div className="flex items-center gap-2 text-caption text-[var(--color-error)] mb-4">
      <AlertTriangle size={12} />
      <span>{msg}</span>
      {onRetry && (
        <IconButton variant="link" size="sm" onClick={onRetry} className="text-[var(--color-error)] underline">
          {t('common.retry')}
        </IconButton>
      )}
    </div>
  );
}
