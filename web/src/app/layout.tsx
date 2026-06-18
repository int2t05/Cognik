/**
 * 根布局 — Apple Design 主题注入 + 全局 Toast。
 */

import type { Metadata } from 'next';
import { ThemeProvider } from '@/components/ThemeProvider';
import { ToastProvider } from '@/hooks/useToast';
import '@/styles/global.css';

export const metadata: Metadata = {
  title: 'OpsMind — 运维数字员工',
  description: 'AI 驱动的企业运维智能助手',
};

// 消除 FOUC：在 HTML 解析前通过 cookie 设置 data-theme
const themeScript = `
  (function() {
    var m = document.cookie.match(/(?:^|;\\s*)theme-preference=([^;]*)/);
    var t = m ? m[1] : (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
    document.documentElement.setAttribute('data-theme', t);
  })();
`.replace(/\s+/g, ' ');

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
        <link
          href="https://fonts.googleapis.com/css2?family=Inter:opsz,wght@14..32,300;14..32,400;14..32,600&display=swap"
          rel="stylesheet"
        />
      </head>
      <body>
        <ThemeProvider>
          <ToastProvider>{children}</ToastProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
