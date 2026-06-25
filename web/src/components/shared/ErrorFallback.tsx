/**
 * ErrorFallback — 统一错误回退组件。
 *
 * admin/error.tsx 和 portal/error.tsx 原有 90% 重复代码（布局、图标、useEffect），
 * 提取后两个 error.tsx 降为 6 行参数化调用。
 */

'use client';

import { useEffect } from 'react';
import { AlertTriangle } from 'lucide-react';
import { AppleButton } from '@/components/ui/AppleButton';

interface ErrorFallbackProps {
  error: Error & { digest?: string };
  reset?: () => void;
  /** 错误标题 */
  title?: string;
  /** 错误描述 */
  message?: string;
  /** 重置按钮文案 */
  resetLabel?: string;
}

export function ErrorFallback({
  error,
  reset,
  title = '页面加载失败',
  message = '请刷新页面重试',
  resetLabel = '刷新页面',
}: ErrorFallbackProps) {
  useEffect(() => {
    console.error('ErrorBoundary caught:', error);
  }, [error]);

  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] gap-4">
      <AlertTriangle size={40} className="text-[var(--color-text-muted-48)]" />
      <h2 className="text-title font-semibold text-[var(--color-ink)]">{title}</h2>
      <p className="text-caption text-[var(--color-text-muted-48)]">{message}</p>
      {reset && (
        <AppleButton variant="pillOutline" onClick={reset}>
          {resetLabel}
        </AppleButton>
      )}
    </div>
  );
}
