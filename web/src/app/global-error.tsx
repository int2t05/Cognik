/**
 * global-error —— 根布局抛错时的兜底页。无法挂 NextIntlClientProvider（provider 在根布局内），
 * 故按 locale cookie 取本地字符串表，最小化双语。
 */

'use client';

import { RotateCw } from 'lucide-react';
import { IconButton } from '@/components/ui/icon-button';

const STRINGS = {
  zh: { title: '系统错误', retry: '重试' },
  en: { title: 'System Error', retry: 'Retry' },
};

function readLocale(): 'zh' | 'en' {
  if (typeof document === 'undefined') return 'en';
  const m = document.cookie.match(/(?:^|;\s*)locale=([^;]*)/);
  return m?.[1] === 'zh' ? 'zh' : 'en';
}

export default function GlobalError({ error, reset }: { error: Error; reset: () => void }) {
  const locale = readLocale();
  const t = STRINGS[locale];
  return (
    <html lang={locale}>
      <body className="m-0 font-sans">
        <div className="min-h-screen flex items-center justify-center bg-[var(--color-parchment)]">
          <div className="text-center max-w-[400px]">
            <h1 className="text-hero font-semibold text-[var(--color-ink)] mb-3">{t.title}</h1>
            <p className="text-body text-[var(--color-text-muted-48)] mb-5">{error.message}</p>
            <IconButton size="lg" onClick={reset}><RotateCw size={18} />{t.retry}</IconButton>
          </div>
        </div>
      </body>
    </html>
  );
}
