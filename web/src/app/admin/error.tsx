'use client';

/** admin/error.tsx — 委托给共享 ErrorFallback。 */
import { ErrorFallback } from '@/components/shared/ErrorFallback';

export default function AdminError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return <ErrorFallback error={error} reset={reset} title="页面加载失败" message="请重试，或返回上一页。如果问题持续存在，请联系管理员。" resetLabel="重试" />;
}
