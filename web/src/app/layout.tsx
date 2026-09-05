/**
 * 根布局 —— 服务端读取 Cookie 注入 data-theme 消除 FOUC；按 locale 设置 <html lang>，
 * NextIntlClientProvider 注入当前 locale 的消息包（按 locale 拆包，服务端加载）。
 */

import type { Metadata } from 'next';
import { cookies } from 'next/headers';
import { NextIntlClientProvider } from 'next-intl';
import { getLocale, getMessages, getTranslations } from 'next-intl/server';
import { Providers } from '@/components/Providers';
import { DynamicTitle } from '@/components/layout/DynamicTitle';
import { getAppName } from '@/lib/config/defaults';
import './globals.css';

/** 按 locale 返回文档标题/描述。 */
export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations('metadata');
  return {
    title: `${getAppName()} — ${t('titleSuffix')}`,
    description: t('description'),
    // favicon 恒定用暗版（浏览器 tab 浅色背景，深色 icon 识别度高，不随主题变）
    icons: { icon: '/icon-dark.svg', apple: '/icon-dark.svg' },
    other: { viewport: 'width=device-width, initial-scale=1, viewport-fit=cover' },
  };
}

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  const cookieStore = await cookies();
  const theme = cookieStore.get('theme-preference')?.value || 'light';
  const locale = await getLocale();
  const messages = await getMessages();

  return (
    <html lang={locale} data-theme={theme} suppressHydrationWarning>
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
        <link
          href="https://fonts.googleapis.com/css2?family=Inter:opsz,wght@14..32,300;14..32,400;14..32,500;14..32,600&display=swap"
          rel="stylesheet"
        />
      </head>
      <body>
        <NextIntlClientProvider locale={locale} messages={messages}>
          <Providers><DynamicTitle />{children}</Providers>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
