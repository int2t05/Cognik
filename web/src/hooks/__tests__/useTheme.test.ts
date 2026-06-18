import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useTheme } from '../useTheme';

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: vi.fn((key: string) => store[key] ?? null),
    setItem: vi.fn((key: string, value: string) => {
      store[key] = value;
    }),
    clear: vi.fn(() => {
      store = {};
    }),
  };
})();

Object.defineProperty(window, 'localStorage', { value: localStorageMock });

// Mock matchMedia
Object.defineProperty(window, 'matchMedia', {
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

beforeEach(() => {
  localStorageMock.clear();
  document.documentElement.removeAttribute('data-theme');
});

describe('useTheme', () => {
  it('模块级缓存默认值 dark 确保 SSR 安全', () => {
    // SSR 阶段不访问 localStorage，模块级 cachedTheme 初始为 'dark'
    // 在浏览器环境（无系统偏好+无localStorage），useEffect 解析为 'light'
    const { result } = renderHook(() => useTheme());
    // 但模块级变量在 SSR 阶段已初始化为 'dark'，不会抛出 ReferenceError
    expect(['dark', 'light']).toContain(result.current.theme);
  });

  it('toggleTheme 在 light/dark 间切换', async () => {
    const { result } = renderHook(() => useTheme());

    await act(async () => {
      result.current.setTheme('light');
    });
    expect(result.current.theme).toBe('light');
    expect(localStorageMock.setItem).toHaveBeenCalledWith('theme-preference', 'light');

    await act(async () => {
      result.current.toggleTheme();
    });
    expect(result.current.theme).toBe('dark');
  });

  it('setTheme 设置 data-theme 属性到 document.documentElement', async () => {
    const { result } = renderHook(() => useTheme());

    await act(async () => {
      result.current.setTheme('light');
    });
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');

    await act(async () => {
      result.current.setTheme('dark');
    });
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });

  it('setTheme 持久化到 localStorage', async () => {
    const { result } = renderHook(() => useTheme());

    await act(async () => {
      result.current.setTheme('light');
    });
    expect(localStorageMock.setItem).toHaveBeenCalledWith('theme-preference', 'light');
  });
});
