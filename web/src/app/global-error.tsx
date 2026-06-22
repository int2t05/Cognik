'use client';

import { AppleButton } from '@/components/ui/AppleButton';

export default function GlobalError({ error, reset }: { error: Error; reset: () => void }) {
  return (
    <html lang="zh-CN">
      <body className="m-0 font-sans">
        <div className="min-h-screen flex items-center justify-center bg-[var(--color-parchment)]">
          <div className="text-center max-w-[400px]">
            <h1 className="text-hero font-medium text-[var(--color-ink)] mb-3">系统错误</h1>
            <p className="text-body text-[var(--color-text-muted-48)] mb-6">{error.message}</p>
            <AppleButton onClick={reset}>重试</AppleButton>
          </div>
        </div>
      </body>
    </html>
  );
}
