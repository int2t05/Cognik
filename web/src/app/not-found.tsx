'use client';

import Link from 'next/link';
import { useTranslations } from 'next-intl';

export default function NotFound() {
  const t = useTranslations();
  return (
    <div className="min-h-screen flex items-center justify-center bg-[var(--color-parchment)]">
      <div className="text-center">
        <h1 className="text-display-md font-light text-[var(--color-ink)] leading-none">404</h1>
        <p className="text-title text-[var(--color-text-muted-48)] mt-2">{t('common.notFound')}</p>
        <Link href="/portal/chat" className="text-[var(--color-accent)] mt-6 inline-block text-title hover:underline transition">{t('common.backHome')}</Link>
      </div>
    </div>
  );
}
