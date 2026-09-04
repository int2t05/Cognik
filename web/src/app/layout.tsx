/**
 * 根布局 — 服务端读取 Cookie 注入 data-theme 消除 FOUC，无需客户端 script。
 */

import type { Metadata } from 'next';
import { cookies } from 'next/headers';
import { Providers } from '@/components/Providers';
import { DynamicTitle } from '@/components/layout/DynamicTitle';
import { getAppName } from '@/lib/config/defaults';
import './globals.css';

export const metadata: Metadata = {
  title: `${getAppName()} — 运维数字员工`,
  description: 'AI 驱动的企业运维智能助手',
  // favicon 恒定用暗版（浏览器 tab 浅色背景，深色 icon 识别度高，不随主题变）
  icons: { icon: '/icon-dark.svg', apple: '/icon-dark.svg' },
  other: { 'viewport': 'width=device-width, initial-scale=1, viewport-fit=cover' },
};

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  const cookieStore = await cookies();
  const theme = cookieStore.get('theme-preference')?.value || 'light';

  return (
    <html lang="zh-CN" data-theme={theme} suppressHydrationWarning>
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
        <link
          href="https://fonts.googleapis.com/css2?family=Inter:opsz,wght@14..32,300;14..32,400;14..32,500;14..32,600&display=swap"
          rel="stylesheet"
        />
      </head>
      <body>
        <Providers><DynamicTitle />{children}</Providers>
      </body>
    </html>
  );
}
