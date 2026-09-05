'use client';

/** admin/error.tsx — 委托给共享 ErrorFallback。 */
import { useTranslations } from 'next-intl';
import { ErrorFallback } from '@/components/shared/ErrorFallback';

export default function AdminError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const t = useTranslations();
  return <ErrorFallback error={error} reset={reset} message={t('error.retryOrBack')} resetLabel={t('common.retry')} />;
}
