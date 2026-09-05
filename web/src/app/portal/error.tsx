'use client';

/** portal/error.tsx — 委托给共享 ErrorFallback。 */
import { useTranslations } from 'next-intl';
import { ErrorFallback } from '@/components/shared/ErrorFallback';

export default function PortalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const t = useTranslations();
  return <ErrorFallback error={error} reset={reset} message={t('error.retryOrHome')} resetLabel={t('common.retry')} />;
}
