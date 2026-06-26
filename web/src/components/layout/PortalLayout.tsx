/** PortalLayout — 门户端布局（顶栏导航 + 内容区）。 */

'use client';

import { useState, useEffect } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { useTheme } from '@/hooks/useTheme';
import { useUnreadCount } from '@/hooks/useUnreadCount';
import { useConfigValue } from '@/hooks/useAppConfig';
import { hasAdminAccess } from '@/lib/roles';
import { AppleButton } from '@/components/ui/AppleButton';
import { AccountSwitcher } from '@/components/shared/AccountSwitcher';
import { MessageSquare, TicketPlus, ListTodo, Bot, Sun, Moon, Shield } from 'lucide-react';

const NAV_ITEMS = [
  { path: '/portal/chat', label: '智能问答', icon: <Bot size={16} /> },
  { path: '/portal/tickets/new', label: '提交申告', icon: <TicketPlus size={16} /> },
  { path: '/portal/tickets', label: '我的申告', icon: <ListTodo size={16} /> },
  { path: '/portal/messages', label: '消息', icon: <MessageSquare size={16} /> },
];

export function PortalLayout({ children }: { children: React.ReactNode }) {
  const { user, permissions } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const { value: appName } = useConfigValue('app_name');
  const pathname = usePathname();
  const router = useRouter();
  const { unreadCount } = useUnreadCount();
  const isAdmin = hasAdminAccess(permissions);
  const [mounted, setMounted] = useState(false);
  useEffect(() => { setMounted(true); }, []);

  return (
    <div className="min-h-screen bg-[var(--color-parchment)]">
      <header className="h-[var(--header-height)] flex items-center justify-between px-6 bg-[var(--color-canvas)]/80 border-b border-[var(--color-hairline)] sticky top-0 z-[var(--z-nav)] backdrop-blur-xl">
        <div className="flex items-center gap-8">
          <button
            aria-label="返回首页"
            onClick={() => router.push('/portal/chat')}
            className="text-headline font-semibold text-[var(--color-ink)] cursor-pointer border-0 bg-transparent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent-focus)] rounded active:scale-95 transition-transform"
          >
            {appName || 'OpsMind'}
          </button>
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
                <div key={item.path} className="relative">
                  <AppleButton
                    variant="menu"
                    icon={item.icon}
                    onClick={() => router.push(item.path)}
                    spanClassName="hidden lg:inline"
                    className={active ? '!bg-[var(--color-divider-soft)] font-semibold' : ''}
                    aria-current={active ? 'page' : undefined}
                  >
                    {item.label}
                  </AppleButton>
                  {item.label === '消息' && unreadCount > 0 && (
                    <span className="absolute -top-0.5 -right-0.5 bg-[var(--color-error)] text-[var(--color-canvas)] w-3 h-3 rounded-full flex items-center justify-center">
                      {unreadCount > 99 ? '99' : unreadCount}
                    </span>
                  )}
                </div>
              );
            })}
          </nav>
        </div>
        <div className="flex items-center gap-3">
          <AppleButton variant="menu" icon={theme === 'dark' ? <Sun /> : <Moon />} onClick={toggleTheme}
            aria-label={theme === 'dark' ? '切换浅色模式' : '切换暗色模式'} />
          {mounted && isAdmin && (
            <AppleButton variant="menu" icon={<Shield />} aria-label="后台管理" onClick={() => router.push('/admin/dashboard')} />
          )}
          {mounted && <span className="text-caption text-[var(--color-text-muted-48)] mr-1">{user?.real_name}</span>}
          {mounted && <AccountSwitcher />}
        </div>
      </header>
      <main className="w-full max-w-wide mx-auto p-6">{children}</main>
    </div>
  );
}
