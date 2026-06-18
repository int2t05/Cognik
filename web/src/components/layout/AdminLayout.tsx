/** AdminLayout — 后台管理布局（侧栏 + 顶栏 + 内容区）。菜单从后端动态渲染。 */

'use client';

import { useState, useEffect } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { useTheme } from '@/hooks/useTheme';
import { useToast } from '@/hooks/useToast';
import { getUnreadCount } from '@/lib/api/message';
import { isActivePath } from '@/lib/menu';
import { AppleButton } from '@/components/ui/AppleButton';

const ICON_MAP: Record<string, string> = {
  dashboard: '📊',
  ticket: '🎫',
  knowledge: '📚',
  users: '👥',
  role: '🔑',
  config: '⚙️',
  audit: '📋',
  message: '💬',
};

export function AdminLayout({ children }: { children: React.ReactNode }) {
  const { user, menus, logout } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const toast = useToast();
  const pathname = usePathname();
  const router = useRouter();
  const [collapsed, setCollapsed] = useState(false);
  const [unreadCount, setUnreadCount] = useState(0);

  // 未读消息轮询（30s）
  useEffect(() => {
    const fetchUnread = () => {
      getUnreadCount().then((d) => setUnreadCount(d.count)).catch(() => {});
    };
    fetchUnread();
    const timer = setInterval(fetchUnread, 30000);
    return () => clearInterval(timer);
  }, []);

  const handleLogout = () => {
    logout();
    router.push('/login');
  };

  return (
    <div style={{ display: 'flex', minHeight: '100vh', background: 'var(--bg-parchment)' }}>
      {/* 侧栏 */}
      <aside
        style={{
          width: collapsed ? 64 : 220,
          minHeight: '100vh',
          background: 'var(--bg-canvas)',
          borderRight: '1px solid var(--hairline)',
          transition: `width var(--duration-normal) var(--ease-out)`,
          display: 'flex',
          flexDirection: 'column',
          position: 'fixed',
          left: 0,
          top: 0,
          bottom: 0,
          zIndex: 100,
        }}
      >
        {/* Logo */}
        <div style={{ padding: '20px 16px', borderBottom: '1px solid var(--divider-soft)' }}>
          <h1 style={{ fontSize: collapsed ? 16 : 18, fontWeight: 600, color: 'var(--text-ink)', whiteSpace: 'nowrap', overflow: 'hidden' }}>
            {collapsed ? 'OM' : 'OpsMind'}
          </h1>
        </div>

        {/* 菜单 */}
        <nav style={{ flex: 1, padding: '8px 0', overflowY: 'auto' }}>
          {menus.map((m) => {
            const active = isActivePath(m.path, pathname);
            return (
              <button
                key={m.id}
                onClick={() => router.push(m.path)}
                title={collapsed ? m.name : undefined}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 12,
                  width: '100%',
                  padding: collapsed ? '12px 0' : '10px 20px',
                  border: 'none',
                  background: active ? 'var(--divider-soft)' : 'transparent',
                  color: active ? 'var(--accent)' : 'var(--text-ink)',
                  fontSize: 14,
                  fontWeight: active ? 600 : 400,
                  cursor: 'pointer',
                  textAlign: 'left',
                  justifyContent: collapsed ? 'center' : 'flex-start',
                  transition: 'background var(--duration-fast)',
                }}
              >
                <span style={{ fontSize: collapsed ? 18 : 16 }}>{ICON_MAP[m.icon] || '📄'}</span>
                {!collapsed && m.name}
              </button>
            );
          })}
        </nav>

        {/* 底部操作 */}
        <div style={{ padding: 12, borderTop: '1px solid var(--divider-soft)', display: 'flex', flexDirection: 'column', gap: 6 }}>
          <button onClick={() => router.push('/portal/messages')} style={sidebarBtnStyle}>
            💬 {!collapsed && <span>消息 {unreadCount > 0 && `(${unreadCount})`}</span>}
          </button>
          <button onClick={toggleTheme} style={sidebarBtnStyle}>
            {theme === 'dark' ? '☀️' : '🌙'} {!collapsed && (theme === 'dark' ? '浅色模式' : '暗色模式')}
          </button>
        </div>
      </aside>

      {/* 主内容区 */}
      <div style={{ flex: 1, marginLeft: collapsed ? 64 : 220, transition: `margin-left var(--duration-normal) var(--ease-out)`, display: 'flex', flexDirection: 'column' }}>
        {/* 顶栏 */}
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
            zIndex: 50,
            backdropFilter: 'saturate(180%) blur(20px)',
          }}
        >
          <button
            onClick={() => setCollapsed(!collapsed)}
            style={{ border: 'none', background: 'none', cursor: 'pointer', fontSize: 18, padding: 4, color: 'var(--text-ink)' }}
          >
            {collapsed ? '→' : '←'}
          </button>
          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
            <span style={{ fontSize: 13, color: 'var(--text-muted-48)' }}>{user?.real_name || user?.username}</span>
            <AppleButton variant="utility" onClick={handleLogout}>登出</AppleButton>
          </div>
        </header>

        {/* 内容 */}
        <main style={{ flex: 1, padding: 24 }}>{children}</main>
      </div>
    </div>
  );
}

const sidebarBtnStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 10,
  padding: '8px 12px',
  border: 'none',
  background: 'transparent',
  color: 'var(--text-muted-80)',
  fontSize: 13,
  cursor: 'pointer',
  borderRadius: 'var(--radius-sm)',
};
