/** PortalLayout — 门户端布局（顶栏导航 + 内容区）。 */

'use client';

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
          <nav className="flex gap-1">
            {NAV_ITEMS.map((item) => {
              const active = pathname === item.path || pathname.startsWith(item.path + '/');
              return (
                <button
                  key={item.path}
                  onClick={() => router.push(item.path)}
                  className={`flex items-center gap-1.5 px-3 py-1 border-0 bg-transparent text-[var(--color-ink)] text-caption rounded-lg cursor-pointer relative transition hover:bg-[var(--color-divider-soft)] ${active ? 'bg-[var(--color-divider-soft)] text-[var(--color-accent)] font-semibold' : ''}`}
                >
                  {item.icon} {item.label}
                  {item.label === '消息' && unreadCount > 0 && (
                    <span className="absolute -top-1 -right-1.5 bg-[var(--color-error)] text-[var(--color-canvas)] text-fine font-semibold w-[18px] h-[18px] rounded-full flex items-center justify-center">
                      {unreadCount > 99 ? '99' : unreadCount}
                    </span>
                  )}
                </button>
              );
            })}
          </nav>
        </div>
        <div className="flex items-center gap-3">
          <button onClick={toggleTheme} aria-label={theme === 'dark' ? '切换浅色模式' : '切换暗色模式'} className="border-0 bg-transparent cursor-pointer p-1 text-[var(--color-ink)] flex">
            {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
          </button>
          {isAdmin && (
            <AppleButton variant="utility" onClick={() => router.push('/admin/dashboard')}>
              <Shield size={14} /> 后台管理
            </AppleButton>
          )}
          <span className="text-caption text-[var(--color-text-muted-48)] mr-1">{user?.real_name}</span>
          <button onClick={async () => { await logout(); router.push('/login'); }} className="flex items-center gap-1.5 border-0 bg-transparent cursor-pointer text-[var(--color-text-muted-48)] text-caption hover:text-[var(--color-ink)] transition">
            <LogOut size={14} /> 登出
          </button>
        </div>
      </header>
      <main className="w-full max-w-wide mx-auto p-6">{children}</main>
    </div>
  );
}
