/** 主题管理 Hook — 双主题切换，useSyncExternalStore 消除 FOUC + SSR 安全。 */

'use client';
import { useSyncExternalStore, useEffect } from 'react';

type Theme = 'light' | 'dark';

function readCookieTheme(): Theme {
  if (typeof document === 'undefined') return 'light';
  const match = document.cookie.match(/(?:^|;\s*)theme-preference=([^;]*)/);
  return match?.[1] === 'light' ? 'light' : 'dark';
}

/** 订阅主题变更（自定义事件 + 跨标签 storage 事件 + 系统偏好变化）。 */
function subscribeTheme(cb: () => void): () => void {
  window.addEventListener('theme-change', cb);
  window.addEventListener('storage', cb);
  const mq = window.matchMedia('(prefers-color-scheme: dark)');
  mq.addEventListener('change', cb);
  return () => {
    window.removeEventListener('theme-change', cb);
    window.removeEventListener('storage', cb);
    mq.removeEventListener('change', cb);
  };
}

/** 读取当前主题快照：localStorage > cookie。 */
function getThemeSnapshot(): Theme {
  const stored = localStorage.getItem('theme-preference');
  if (stored === 'light' || stored === 'dark') return stored;
  return readCookieTheme();
}

/** SSR 快照 — 固定返回 light，与客户端首次渲染一致，避免 hydration 不匹配。 */
function getThemeServerSnapshot(): Theme {
  return 'light';
}

export function useTheme() {
  const theme = useSyncExternalStore(subscribeTheme, getThemeSnapshot, getThemeServerSnapshot);

  // 同步 DOM 属性 + cookie + localStorage（不触发 setState，无 lint 风险）
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    document.cookie = `theme-preference=${theme}; path=/; max-age=${365 * 86400}; SameSite=Lax`;
    localStorage.setItem('theme-preference', theme);
  }, [theme]);

  const toggleTheme = () => {
    const next = theme === 'light' ? 'dark' : 'light';
    localStorage.setItem('theme-preference', next);
    window.dispatchEvent(new Event('theme-change'));
  };

  const setTheme = (t: Theme) => {
    localStorage.setItem('theme-preference', t);
    window.dispatchEvent(new Event('theme-change'));
  };

  return { theme, toggleTheme, setTheme };
}
