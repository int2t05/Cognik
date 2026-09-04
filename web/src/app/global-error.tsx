'use client';

import { RotateCw } from 'lucide-react';
import { IconButton } from '@/components/ui/icon-button';

export default function GlobalError({ error, reset }: { error: Error; reset: () => void }) {
  return (
    <html lang="zh-CN">
      <body className="m-0 font-sans">
        <div className="min-h-screen flex items-center justify-center bg-[var(--color-parchment)]">
          <div className="text-center max-w-[400px]">
            <h1 className="text-hero font-semibold text-[var(--color-ink)] mb-3">系统错误</h1>
            <p className="text-body text-[var(--color-text-muted-48)] mb-5">{error.message}</p>
            <IconButton size="lg" onClick={reset}><RotateCw size={18} />重试</IconButton>
          </div>
        </div>
      </body>
    </html>
  );
}
