/**
 * AccountSwitcher — 切换账号弹出框。
 *
 * 点击「切换账号」按钮后弹出，列出历史登录会话。
 * 有效会话（7 天内）点击直接切换，过期或新增账号跳转登录页。
 */
'use client';

import { useState, useRef, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { UserPlus, Trash2, LogIn } from 'lucide-react';
import { useAccountSwitcher } from '@/hooks/useAccountSwitcher';

interface Props {
  /** 触发按钮的 className（由调用方控制样式）。 */
  className?: string;
  /** 是否仅显示图标（折叠态）。 */
  iconOnly?: boolean;
}

export function AccountSwitcher({ className, iconOnly }: Props) {
  const { accounts, switchTo, removeAccount, logout } = useAccountSwitcher();
  const [open, setOpen] = useState(false);
  const router = useRouter();
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  const handleSwitch = (account: (typeof accounts)[0]) => {
    const ok = switchTo(account);
    setOpen(false);
    if (ok) {
      router.push('/portal/chat');
    } else {
      logout();
      router.push('/login');
    }
  };

  const handleNewLogin = () => {
    setOpen(false);
    logout();
    router.push('/login');
  };

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        aria-label="切换账号"
        className={className || 'flex items-center gap-1.5 border-0 bg-transparent cursor-pointer text-[var(--color-text-muted-48)] text-caption hover:text-[var(--color-ink)] transition min-h-[44px] min-w-[44px] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent-focus)]'}
      >
        <UserPlus size={16} />
        {!iconOnly && <span>切换账号</span>}
      </button>

      {open && (
        <div className="absolute right-0 top-full mt-2 w-64 bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)] shadow-[var(--shadow-dialog)] z-50 overflow-hidden">
          <div className="px-4 py-3 border-b border-[var(--color-divider-soft)]">
            <p className="text-fine text-[var(--color-text-muted-48)]">切换账号</p>
          </div>

          <div className="max-h-[280px] overflow-y-auto">
            {accounts.length === 0 ? (
              <p className="px-4 py-6 text-caption text-[var(--color-text-muted-48)] text-center">
                暂无历史账号
              </p>
            ) : (
              accounts.map((a) => {
                const expired = Date.now() - a.savedAt > 7 * 24 * 3600 * 1000;
                return (
                  <button
                    key={a.username}
                    onClick={() => handleSwitch(a)}
                    className={`w-full flex items-center gap-3 px-4 py-3 text-left border-0 bg-transparent cursor-pointer transition hover:bg-[var(--color-divider-soft)] text-caption ${expired ? 'opacity-50' : ''}`}
                  >
                    <span className="w-8 h-8 rounded-full bg-[var(--color-accent)]/10 flex items-center justify-center text-caption font-semibold text-[var(--color-accent)] shrink-0">
                      {a.realName?.[0] || a.username?.[0] || '?'}
                    </span>
                    <span className="flex-1 min-w-0">
                      <span className="block truncate text-[var(--color-ink)]">{a.realName || a.username}</span>
                      <span className="block text-fine text-[var(--color-text-muted-48)]">
                        {a.username}{expired ? ' · 已过期' : ''}
                      </span>
                    </span>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        removeAccount(a.username);
                      }}
                      aria-label={`移除 ${a.username}`}
                      className="p-1 border-0 bg-transparent cursor-pointer text-[var(--color-text-muted-48)] hover:text-[var(--color-error)] transition"
                    >
                      <Trash2 size={14} />
                    </button>
                  </button>
                );
              })
            )}
          </div>

          <div className="border-t border-[var(--color-divider-soft)]">
            <button
              onClick={handleNewLogin}
              className="w-full flex items-center gap-3 px-4 py-3 text-left border-0 bg-transparent cursor-pointer transition hover:bg-[var(--color-divider-soft)] text-caption text-[var(--color-accent)] font-semibold"
            >
              <LogIn size={16} />
              其他账号登录
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
