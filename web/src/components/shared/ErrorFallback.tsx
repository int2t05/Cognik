/**
 * ErrorFallback — 统一错误回退组件。
 * admin/error.tsx 和 portal/error.tsx 参数化调用本组件。
 */

'use client';

import { useEffect } from 'react';
import { useTranslations } from 'next-intl';
import { AlertTriangle } from 'lucide-react';
import { IconButton } from '@/components/ui/icon-button';

interface ErrorFallbackProps {
  error: Error & { digest?: string };
  reset?: () => void;
  /** 错误标题；不传则按 locale 翻译。 */
  title?: string;
  /** 错误描述；不传则按 locale 翻译。 */
  message?: string;
  /** 重置按钮文案；不传则按 locale 翻译。 */
  resetLabel?: string;
}

export function ErrorFallback({
  error,
  reset,
  title,
  message,
  resetLabel,
}: ErrorFallbackProps) {
  const t = useTranslations();
  useEffect(() => {
    console.error('ErrorBoundary caught:', error);
  }, [error]);

  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] gap-4">
      <AlertTriangle size={40} className="text-[var(--color-text-muted-48)]" />
      <h2 className="text-title font-semibold text-[var(--color-ink)]">{title ?? t('error.pageLoadFailed')}</h2>
      <p className="text-caption text-[var(--color-text-muted-48)]">{message ?? t('error.refreshHint')}</p>
      {reset && (
        <IconButton variant="outline" size="lg" onClick={reset}>
          {resetLabel ?? t('error.refreshPage')}
        </IconButton>
      )}
    </div>
  );
}
