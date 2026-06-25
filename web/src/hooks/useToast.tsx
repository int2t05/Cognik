/** 全局 Toast 系统 — 按类型分色 + 图标，最多堆叠 3 条，点击即关闭。 */

'use client';
import { createContext, useContext, useState, useCallback, useRef, type ReactNode } from 'react';
import { CheckCircle, XCircle, AlertTriangle, Info } from 'lucide-react';

type ToastType = 'success' | 'error' | 'warning' | 'info';

interface Toast {
  id: number;
  type: ToastType;
  message: string;
}

interface ToastContextValue {
  toasts: Toast[];
  success: (message: string) => void;
  error: (message: string) => void;
  warning: (message: string) => void;
  info: (message: string) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

const TOAST_DURATION: Record<ToastType, number> = { error: 5000, warning: 4000, success: 3000, info: 3000 };

/** 每种类型的色值 + 图标映射 */
const TOAST_STYLE: Record<ToastType, { icon: ReactNode; bg: string; border: string; text: string }> = {
  success: { icon: <CheckCircle className="h-4 w-4" />, bg: 'var(--badge-success-bg)', border: 'var(--color-success)', text: 'var(--badge-success-text)' },
  error:   { icon: <XCircle className="h-4 w-4" />,     bg: 'var(--badge-error-bg)',   border: 'var(--color-error)',   text: 'var(--badge-error-text)' },
  warning: { icon: <AlertTriangle className="h-4 w-4" />, bg: 'var(--badge-warning-bg)', border: 'var(--color-warning)', text: 'var(--badge-warning-text)' },
  info:    { icon: <Info className="h-4 w-4" />,          bg: 'var(--color-tile-1)',     border: 'var(--color-info)',    text: 'var(--color-ink)' },
};

let nextId = 0;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const timers = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map());

  const dismiss = useCallback((id: number) => {
    const t = timers.current.get(id);
    if (t) { clearTimeout(t); timers.current.delete(id); }
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const addToast = useCallback((type: ToastType, message: string) => {
    const id = ++nextId;
    setToasts((prev) => [...prev.slice(-2), { id, type, message }]);
    timers.current.set(id, setTimeout(() => dismiss(id), TOAST_DURATION[type]));
  }, [dismiss]);

  const success = useCallback((msg: string) => addToast('success', msg), [addToast]);
  const error   = useCallback((msg: string) => addToast('error', msg),   [addToast]);
  const warning = useCallback((msg: string) => addToast('warning', msg), [addToast]);
  const info    = useCallback((msg: string) => addToast('info', msg),    [addToast]);

  return (
    <ToastContext.Provider value={{ toasts, success, error, warning, info }}>
      {children}
      <div role="region" aria-label="通知" aria-live="polite"
        className="fixed top-4 right-4 z-[var(--z-toast)] flex flex-col gap-2 pointer-events-none">
        {toasts.map((t) => {
          const s = TOAST_STYLE[t.type];
          return (
            <div key={t.id} role="alert" onClick={() => dismiss(t.id)}
              className="flex items-center gap-2.5 px-3.5 py-2.5 text-caption rounded-[var(--radius-apple)] shadow-[var(--shadow-dialog)] backdrop-blur-xl max-w-[360px] pointer-events-auto animate-[fadeIn_0.2s_ease-out] cursor-pointer active:scale-[0.98] transition"
              style={{ background: s.bg, color: s.text, border: `1px solid ${s.border}`, borderLeft: `3px solid ${s.border}` }}>
              <span className="flex-shrink-0" style={{ color: s.border }}>{s.icon}</span>
              <span className="flex-1">{t.message}</span>
            </div>
          );
        })}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used within ToastProvider');
  return ctx;
}
