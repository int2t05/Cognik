/** PortalLayout — 门户端布局（顶栏导航 + 内容区）。 */

'use client';

import { useState, useEffect } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { useTheme } from '@/hooks/useTheme';
import { getUnreadCount } from '@/lib/api/message';
import { AppleButton } from '@/components/ui/AppleButton';
import { MessageSquare, TicketPlus, ListTodo, Bot, Sun, Moon, Shield, LogOut } from 'lucide-react';
import styles from './PortalLayout.module.css';

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
  const [unreadCount, setUnreadCount] = useState(0);
  const isAdmin = menus.length > 0;

  useEffect(() => {
    const fetch = () => { getUnreadCount().then((d) => setUnreadCount(d.count)).catch(() => {}); };
    fetch();
    const t = setInterval(fetch, 30000);
    return () => clearInterval(t);
  }, []);

  return (
    <div className={styles.layout}>
      <header className={styles.topbar}>
        <div className={styles.brand}>
          <span
            role="button"
            tabIndex={0}
            aria-label="返回首页"
            onClick={() => router.push('/portal/chat')}
            onKeyDown={(e) => { if (e.key === 'Enter') router.push('/portal/chat'); }}
            className={styles.logo}
          >
            OpsMind
          </span>
          <nav className={styles.nav}>
            {NAV_ITEMS.map((item) => {
              const active = pathname === item.path || pathname.startsWith(item.path + '/');
              return (
                <button
                  key={item.path}
                  onClick={() => router.push(item.path)}
                  className={`${styles.navItem} ${active ? styles.navItemActive : ''}`}
                >
                  {item.icon} {item.label}
                  {item.label === '消息' && unreadCount > 0 && (
                    <span className={styles.navBadge}>
                      {unreadCount > 99 ? '99' : unreadCount}
                    </span>
                  )}
                </button>
              );
            })}
          </nav>
        </div>
        <div className={styles.actions}>
          <button onClick={toggleTheme} aria-label={theme === 'dark' ? '切换浅色模式' : '切换暗色模式'} className={styles.themeBtn}>
            {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
          </button>
          {isAdmin && (
            <AppleButton variant="utility" onClick={() => router.push('/admin/dashboard')}>
              <Shield size={14} className={styles.shieldIcon} /> 后台管理
            </AppleButton>
          )}
          <span className={styles.userName}>{user?.real_name}</span>
          <button onClick={() => { logout(); router.push('/login'); }} className={styles.logoutBtn}>
            <LogOut size={14} /> 登出
          </button>
        </div>
      </header>
      <main className={styles.content}>{children}</main>
    </div>
  );
}
