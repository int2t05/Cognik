/**
 * 根布局 — 服务端读取 Cookie 注入 data-theme 消除 FOUC，无需客户端 script。
 */

import type { Metadata } from 'next';
import { cookies } from 'next/headers';
import { Providers } from '@/components/Providers';
import './globals.css';

export const metadata: Metadata = {
  title: 'OpsMind — 运维数字员工',
  description: 'AI 驱动的企业运维智能助手',
  icons: { icon: '/icon-64.png', apple: '/icon-180.png' },
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
          href="https://fonts.googleapis.com/css2?family=Inter:opsz,wght@14..32,300;14..32,400;14..32,600&display=swap"
          rel="stylesheet"
        />
      </head>
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
