/**
 * 主题提供者 — 在客户端注入 data-theme 属性并应用 CSS 变量。
 * 使用 'use client' 因为需要访问 localStorage 和 document。
 */

'use client';

import { useEffect, type ReactNode } from 'react';
import { useTheme } from '@/hooks/useTheme';

export function ThemeProvider({ children }: { children: ReactNode }) {
  const { theme } = useTheme();

  // 确保 data-theme 在初始渲染后被设置
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
  }, [theme]);

  return <>{children}</>;
}
