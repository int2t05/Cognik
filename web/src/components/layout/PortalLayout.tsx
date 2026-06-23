/** PortalLayout — 门户端布局（顶栏导航 + 内容区）。 */

'use client';

import { useState, useEffect } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { useTheme } from '@/hooks/useTheme';
import { useUnreadCount } from '@/hooks/useUnreadCount';
import { AppleButton } from '@/components/ui/AppleButton';
import { MessageSquare, TicketPlus, ListTodo, Bot, Sun, Moon, Shield, LogOut } from 'lucide-react';

const NAV_ITEMS = [
  { path: '/portal/chat', label: '智能问答', icon: <Bot size={16} /> },
  { path: '/portal/tickets/new', label: '提交申告', icon: <TicketPlus size={16} /> },
  { path: '/portal/tickets', label: '我的申告', icon: <ListTodo size={16} /> },
  { path: '/portal/messages', label: '消息', icon: <MessageSquare size={16} /> },
];

export function PortalLayout({ children }: { children: React.ReactNode }) {
  const { user, logout, menus } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const pathname = usePathname();
  const router = useRouter();
  const { unreadCount } = useUnreadCount();
  const isAdmin = menus.length > 0;
  const [mounted, setMounted] = useState(false);
  useEffect(() => { setMounted(true); }, []);

  return (
    <div className="min-h-screen bg-[var(--color-parchment)]">
      <header className="h-[var(--header-height)] flex items-center justify-between px-6 bg-[var(--color-canvas)]/80 border-b border-[var(--color-hairline)] sticky top-0 z-[var(--z-nav)] backdrop-blur-xl">
        <div className="flex items-center gap-8">
          <span
            role="button"
            tabIndex={0}
            aria-label="返回首页"
            onClick={() => router.push('/portal/chat')}
            onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); router.push('/portal/chat'); } }}
            className="text-headline font-semibold text-[var(--color-ink)] cursor-pointer border-0 bg-transparent"
          >
            OpsMind
          </span>
          <nav className="flex gap-2">
            {NAV_ITEMS.map((item) => {
              // 严格匹配：/portal/tickets 匹配自身和 /portal/tickets/123，但不匹配 /portal/tickets/new
              const active = (() => {
                if (pathname === item.path) return true;
                if (!pathname.startsWith(item.path + '/')) return false;
                // 如果存在另一个导航项精确匹配当前路径，则当前项不应激活
                const exactMatch = NAV_ITEMS.some(
                  (other) => other.path !== item.path && other.path === pathname,
                );
                return !exactMatch;
              })();
              return (
                <button
                  key={item.path}
                  onClick={() => router.push(item.path)}
                  className={`flex items-center gap-1.5 px-3 py-3 min-h-[44px] border-0 bg-transparent text-[var(--color-ink)] text-caption rounded-[var(--radius-pill)] cursor-pointer relative transition active:scale-95 hover:bg-[var(--color-divider-soft)] ${active ? 'bg-[var(--color-divider-soft)] font-semibold shadow-[inset_0_-2px_0_var(--color-accent)]' : ''}`}
                >
                  {item.icon} {item.label}
                  {item.label === '消息' && unreadCount > 0 && (
                    <span className="absolute -top-1 -right-1.5 bg-[var(--color-error)] text-[var(--color-canvas)] text-fine font-semibold w-5 h-5 rounded-full flex items-center justify-center">
                      {unreadCount > 99 ? '99' : unreadCount}
                    </span>
                  )}
                </button>
              );
            })}
          </nav>
        </div>
        <div className="flex items-center gap-3">
          <button onClick={toggleTheme} aria-label={theme === 'dark' ? '切换浅色模式' : '切换暗色模式'} className="border-0 bg-transparent cursor-pointer p-3 text-[var(--color-ink)] flex transition hover:opacity-70 min-h-[44px] min-w-[44px] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent-focus)]">
            {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
          </button>
          {mounted && isAdmin && (
            <AppleButton variant="utility" className="p-3.5" aria-label="后台管理" onClick={() => router.push('/admin/dashboard')}>
              <Shield size={16} />
            </AppleButton>
          )}
          <span className="text-caption text-[var(--color-text-muted-48)] mr-1" suppressHydrationWarning>{user?.real_name}</span>
          <button onClick={async () => { await logout(); router.push('/login'); }} aria-label="登出" className="flex items-center border-0 bg-transparent cursor-pointer text-[var(--color-text-muted-48)] p-3 hover:text-[var(--color-ink)] transition min-h-[44px] min-w-[44px] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent-focus)]">
            <LogOut size={16} />
          </button>
        </div>
      </header>
      <main className="w-full max-w-wide mx-auto p-6">{children}</main>
    </div>
  );
}
