/**
 * 主题管理 Hook — SSR 安全。
 *
 * 修复旧版模块级 localStorage 访问导致的 SSR 不兼容问题。
 * 参照 docs/TODO.md P1-19。
 */

'use client';
import { useState, useEffect, useCallback } from 'react';

type Theme = 'light' | 'dark';

// 模块级缓存，SSR 安全（不在模块顶层访问 localStorage）
let cachedTheme: Theme = 'dark';

export function useTheme() {
  const [theme, setThemeState] = useState<Theme>(cachedTheme);

  useEffect(() => {
    // 客户端 hydration：从 localStorage 读取
    const stored = localStorage.getItem('theme-preference') as Theme | null;
    const resolved: Theme =
      stored ||
      (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
    applyTheme(resolved);
  }, []);

  const applyTheme = useCallback((t: Theme) => {
    cachedTheme = t;
    setThemeState(t);
    document.documentElement.setAttribute('data-theme', t);
    localStorage.setItem('theme-preference', t);
  }, []);

  const toggleTheme = useCallback(() => {
    applyTheme(cachedTheme === 'light' ? 'dark' : 'light');
  }, [applyTheme]);

  const setTheme = useCallback(
    (t: Theme) => {
      applyTheme(t);
    },
    [applyTheme]
  );

  return { theme, toggleTheme, setTheme };
}
