/** 全局 Toast 系统 — 统一消息通知，最多堆叠 3 条。 */
/** Toast 消失时间按类型分级：error 5s，warning 4s，success/info 3s。 */

'use client';
import {
  createContext,
  useContext,
  useState,
  useCallback,
  type ReactNode,
} from 'react';

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

/** 按类型分级消失时间（ms） */
const TOAST_DURATION: Record<ToastType, number> = {
  error: 5000,
  warning: 4000,
  success: 3000,
  info: 3000,
};

let nextId = 0;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const addToast = useCallback((type: ToastType, message: string) => {
    const id = ++nextId;
    setToasts((prev) => [...prev.slice(-2), { id, type, message }]); // 最多堆叠 3 条
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, TOAST_DURATION[type]);
  }, []);

  const success = useCallback((msg: string) => addToast('success', msg), [addToast]);
  const error = useCallback((msg: string) => addToast('error', msg), [addToast]);
  const warning = useCallback((msg: string) => addToast('warning', msg), [addToast]);
  const info = useCallback((msg: string) => addToast('info', msg), [addToast]);

  return (
    <ToastContext.Provider value={{ toasts, success, error, warning, info }}>
      {children}
      {/* Toast 容器 — 右上角固定 */}
      <div
        role="region"
        aria-label="通知"
        style={{
          position: 'fixed',
          top: 16,
          right: 16,
          zIndex: 'var(--z-toast)',
          display: 'flex',
          flexDirection: 'column',
          gap: 8,
        }}
      >
        {toasts.map((t) => (
          <div
            key={t.id}
            role="alert"
            style={{
              background: 'var(--color-parchment)',
              color: 'var(--color-ink)',
              padding: '12px 20px',
              borderRadius: 'var(--radius-md)',
              fontSize: 14,
              fontWeight: 500,
              boxShadow: '0 2px 12px rgba(0,0,0,0.12)',
              backdropFilter: 'blur(20px)',
              animation: 'fadeIn 0.25s ease-out',
              maxWidth: 360,
            }}
          >
            {t.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used within ToastProvider');
  return ctx;
}
