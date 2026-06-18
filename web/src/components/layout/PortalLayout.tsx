/** PortalLayout — 门户端布局（顶栏导航 + 内容区）。 */

'use client';

import { useState, useEffect } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { useTheme } from '@/hooks/useTheme';
import { getUnreadCount } from '@/lib/api/message';
import { AppleButton } from '@/components/ui/AppleButton';

const NAV_ITEMS = [
  { path: '/portal/chat', label: '智能问答' },
  { path: '/portal/tickets/new', label: '提交申告' },
  { path: '/portal/tickets', label: '我的申告' },
  { path: '/portal/messages', label: '消息' },
];

export function PortalLayout({ children }: { children: React.ReactNode }) {
  const { user, logout, menus } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const pathname = usePathname();
  const router = useRouter();
  const [unreadCount, setUnreadCount] = useState(0);

  // 消息轮询
  useEffect(() => {
    const fetchUnread = () => {
      getUnreadCount().then((d) => setUnreadCount(d.count)).catch(() => {});
    };
    fetchUnread();
    const timer = setInterval(fetchUnread, 30000);
    return () => clearInterval(timer);
  }, []);

  const isAdmin = menus.length > 0;

  return (
    <div style={{ minHeight: '100vh', background: 'var(--bg-parchment)' }}>
      {/* 顶栏 — Apple sub-nav frosted 风格 */}
      <header
        style={{
          height: 52,
          background: 'var(--bg-canvas)',
          borderBottom: '1px solid var(--hairline)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '0 24px',
          position: 'sticky',
          top: 0,
          zIndex: 100,
          backdropFilter: 'saturate(180%) blur(20px)',
        }}
      >
        {/* 左侧：品牌 + 导航 */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 32 }}>
          <span
            style={{ fontSize: 21, fontWeight: 600, letterSpacing: '0.231px', color: 'var(--text-ink)', cursor: 'pointer' }}
            onClick={() => router.push('/portal/chat')}
          >
            OpsMind
          </span>
          <nav style={{ display: 'flex', gap: 4 }}>
            {NAV_ITEMS.map((item) => {
              const active = pathname === item.path || pathname.startsWith(item.path + '/');
              return (
                <button
                  key={item.path}
                  onClick={() => router.push(item.path)}
                  style={{
                    padding: '6px 14px',
                    border: 'none',
                    background: active ? 'var(--divider-soft)' : 'transparent',
                    color: active ? 'var(--accent)' : 'var(--text-ink)',
                    fontSize: 14,
                    fontWeight: active ? 600 : 400,
                    borderRadius: 'var(--radius-sm)',
                    cursor: 'pointer',
                    position: 'relative',
                  }}
                >
                  {item.label}
                  {item.label === '消息' && unreadCount > 0 && (
                    <span style={{
                      position: 'absolute',
                      top: -4,
                      right: -6,
                      background: 'var(--color-error)',
                      color: '#fff',
                      fontSize: 10,
                      fontWeight: 600,
                      width: 18,
                      height: 18,
                      borderRadius: '50%',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                    }}>
                      {unreadCount > 99 ? '99' : unreadCount}
                    </span>
                  )}
                </button>
              );
            })}
          </nav>
        </div>

        {/* 右侧：操作 */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <button onClick={toggleTheme} style={{ border: 'none', background: 'none', cursor: 'pointer', fontSize: 16 }}>
            {theme === 'dark' ? '☀️' : '🌙'}
          </button>
          {isAdmin && (
            <AppleButton variant="utility" onClick={() => router.push('/admin/dashboard')}>
              后台管理
            </AppleButton>
          )}
          <span style={{ fontSize: 13, color: 'var(--text-muted-48)' }}>{user?.real_name}</span>
          <AppleButton variant="utility" onClick={() => { logout(); router.push('/login'); }}>登出</AppleButton>
        </div>
      </header>

      {/* 内容区 */}
      <main style={{ maxWidth: 1200, margin: '0 auto', padding: 24 }}>{children}</main>
    </div>
  );
}
