/** 主题管理 Hook — 双主题切换，cookie 预读消除 FOUC。 */

'use client';
import { useState, useEffect, useCallback } from 'react';

type Theme = 'light' | 'dark';

function readCookieTheme(): Theme {
  if (typeof document === 'undefined') return 'dark';
  const match = document.cookie.match(/(?:^|;\s*)theme-preference=([^;]*)/);
  return match?.[1] === 'light' ? 'light' : 'dark';
}

let cachedTheme: Theme = 'dark';

export function useTheme() {
  const [theme, setThemeState] = useState<Theme>(cachedTheme);

  useEffect(() => {
    const stored = localStorage.getItem('theme-preference') as Theme | null;
    const resolved: Theme = stored || readCookieTheme() ||
      (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
    applyTheme(resolved);
  }, []);

  const applyTheme = useCallback((t: Theme) => {
    cachedTheme = t;
    setThemeState(t);
    document.documentElement.setAttribute('data-theme', t);
    document.cookie = `theme-preference=${t}; path=/; max-age=${365 * 86400}; SameSite=Lax`;
    localStorage.setItem('theme-preference', t);
  }, []);

  const toggleTheme = useCallback(() => {
    applyTheme(cachedTheme === 'light' ? 'dark' : 'light');
  }, [applyTheme]);

  return { theme, toggleTheme, setTheme: applyTheme };
}
