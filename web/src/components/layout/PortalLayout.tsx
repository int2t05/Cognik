/** PortalLayout — 门户端布局（顶栏导航 + 内容区）。 */

'use client';

import { useState, useEffect } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { useTheme } from '@/hooks/useTheme';
import { getUnreadCount } from '@/lib/api/message';
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
  const [unreadCount, setUnreadCount] = useState(0);
  const isAdmin = menus.length > 0;

  useEffect(() => {
    const fetch = () => { getUnreadCount().then((d) => setUnreadCount(d.count)).catch(() => {}); };
    fetch();
    const t = setInterval(fetch, 30000);
    return () => clearInterval(t);
  }, []);

  return (
    <div style={{ minHeight: '100vh', background: 'var(--bg-parchment)' }}>
      <header style={{
        height: 52, background: 'var(--bg-canvas)', borderBottom: '1px solid var(--hairline)',
        display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 24px',
        position: 'sticky', top: 0, zIndex: 100, backdropFilter: 'saturate(180%) blur(20px)',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 32 }}>
          <span
            role="button"
            tabIndex={0}
            aria-label="返回首页"
            onClick={() => router.push('/portal/chat')}
            onKeyDown={(e) => { if (e.key === 'Enter') router.push('/portal/chat'); }}
            style={{ fontSize: 21, fontWeight: 600, letterSpacing: '0.231px', color: 'var(--text-ink)', cursor: 'pointer' }}
          >
            OpsMind
          </span>
          <nav style={{ display: 'flex', gap: 4 }}>
            {NAV_ITEMS.map((item) => {
              const active = pathname === item.path || pathname.startsWith(item.path + '/');
              return (
                <button key={item.path} onClick={() => router.push(item.path)} style={{
                  display: 'flex', alignItems: 'center', gap: 6, padding: '6px 14px',
                  border: 'none', background: active ? 'var(--divider-soft)' : 'transparent',
                  color: active ? 'var(--accent)' : 'var(--text-ink)', fontSize: 14,
                  fontWeight: active ? 600 : 400, borderRadius: 'var(--radius-sm)', cursor: 'pointer', position: 'relative',
                }}>
                  {item.icon} {item.label}
                  {item.label === '消息' && unreadCount > 0 && (
                    <span style={{ position: 'absolute', top: -4, right: -6, background: 'var(--color-error)', color: '#fff', fontSize: 10, fontWeight: 600, width: 18, height: 18, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                      {unreadCount > 99 ? '99' : unreadCount}
                    </span>
                  )}
                </button>
              );
            })}
          </nav>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <button onClick={toggleTheme} aria-label={theme === 'dark' ? '切换浅色模式' : '切换暗色模式'} style={{ border: 'none', background: 'none', cursor: 'pointer', color: 'var(--text-ink)', display: 'flex', padding: 4 }}>
            {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
          </button>
          {isAdmin && (
            <AppleButton variant="utility" onClick={() => router.push('/admin/dashboard')}>
              <Shield size={14} style={{ marginRight: 4 }} /> 后台管理
            </AppleButton>
          )}
          <span style={{ fontSize: 13, color: 'var(--text-muted-48)' }}>{user?.real_name}</span>
          <button onClick={() => { logout(); router.push('/login'); }} style={{ border: 'none', background: 'none', cursor: 'pointer', color: 'var(--text-muted-48)', display: 'flex', alignItems: 'center', gap: 4, fontSize: 13 }}>
            <LogOut size={14} /> 登出
          </button>
        </div>
      </header>
      <main style={{ maxWidth: 1200, margin: '0 auto', padding: 24 }}>{children}</main>
    </div>
  );
}
