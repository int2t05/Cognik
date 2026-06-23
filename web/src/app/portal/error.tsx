'use client';

/**
 * portal/error.tsx — 门户端 Error Boundary。
 *
 * 捕获门户路由下未处理的渲染错误，提供重试入口。
 */
import { useEffect } from 'react';
import { AppleButton } from '@/components/ui/AppleButton';
import { AlertTriangle } from 'lucide-react';

export default function PortalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error('Portal page error:', error);
  }, [error]);

  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] gap-[var(--spacing-md-plus)]">
      <AlertTriangle className="w-12 h-12 text-[var(--color-warning)]" />
      <div className="text-center">
        <h2 className="text-headline font-semibold text-[var(--color-ink)] mb-2">
          页面加载失败
        </h2>
        <p className="text-caption text-[var(--color-text-muted-48)] max-w-[320px]">
          请重试，或返回首页。如果问题持续存在，请联系管理员。
        </p>
      </div>
      <AppleButton variant="pill" onClick={reset}>
        重试
      </AppleButton>
    </div>
  );
}
