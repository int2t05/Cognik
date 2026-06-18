/** AdminLayout — 后台管理布局（侧栏嵌套菜单 + 顶栏 + 内容区）。 */

'use client';

import { useState, useEffect } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { useTheme } from '@/hooks/useTheme';
import { getUnreadCount } from '@/lib/api/message';
import { isActivePath } from '@/lib/menu';
import { AppleButton } from '@/components/ui/AppleButton';
import { LayoutDashboard, Ticket, BookOpen, Users, Shield, Settings, ScrollText, MessageSquare, ChevronLeft, ChevronRight, Sun, Moon, LogOut, ChevronDown } from 'lucide-react';

const ICON_MAP: Record<string, React.ReactNode> = {
  dashboard: <LayoutDashboard size={18} />,
  ticket: <Ticket size={18} />,
  knowledge: <BookOpen size={18} />,
  users: <Users size={18} />,
  role: <Shield size={18} />,
  config: <Settings size={18} />,
  audit: <ScrollText size={18} />,
  message: <MessageSquare size={18} />,
};

interface MenuItem { id: number; name: string; path: string; icon: string; parent_id: number; sort_order: number; type: string; children?: MenuItem[]; }

export function AdminLayout({ children }: { children: React.ReactNode }) {
  const { user, menus, logout } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const pathname = usePathname();
  const router = useRouter();
  const [collapsed, setCollapsed] = useState(() => {
    if (typeof window !== 'undefined') return localStorage.getItem('sidebar-collapsed') === 'true';
    return false;
  });
  const [expandedMenus, setExpandedMenus] = useState<Set<number>>(new Set());
  const [unreadCount, setUnreadCount] = useState(0);

  useEffect(() => {
    localStorage.setItem('sidebar-collapsed', String(collapsed));
  }, [collapsed]);

  // 消息轮询
  useEffect(() => {
    const fetch = () => { getUnreadCount().then((d) => setUnreadCount(d.count)).catch(() => {}); };
    fetch();
    const t = setInterval(fetch, 30000);
    return () => clearInterval(t);
  }, []);

  const toggleSubmenu = (id: number) => {
    setExpandedMenus((prev) => { const next = new Set(prev); if (next.has(id)) next.delete(id); else next.add(id); return next; });
  };

  const renderMenuItem = (m: MenuItem, depth = 0) => {
    const hasChildren = m.children && m.children.length > 0;
    const active = isActivePath(m.path, pathname);
    const expanded = expandedMenus.has(m.id);

    return (
      <div key={m.id}>
        <button
          onClick={() => { if (hasChildren) toggleSubmenu(m.id); else router.push(m.path); }}
          title={collapsed ? m.name : undefined}
          style={{
            display: 'flex', alignItems: 'center', gap: 12, width: '100%',
            padding: collapsed ? '12px 0' : `10px ${20 + depth * 16}px`,
            border: 'none', background: active ? 'var(--divider-soft)' : 'transparent',
            color: active ? 'var(--accent)' : 'var(--text-ink)', fontSize: 14,
            fontWeight: active ? 600 : 400, cursor: 'pointer', textAlign: 'left' as const,
            justifyContent: collapsed ? 'center' : 'flex-start',
          }}
        >
          {ICON_MAP[m.icon] || <Settings size={18} />}
          {!collapsed && <span style={{ flex: 1 }}>{m.name}</span>}
          {!collapsed && hasChildren && <ChevronDown size={14} style={{ transform: expanded ? 'rotate(180deg)' : '', transition: 'transform 0.2s' }} />}
        </button>
        {!collapsed && hasChildren && expanded && m.children!.map((c) => renderMenuItem(c, depth + 1))}
      </div>
    );
  };

  const topMenus = menus.filter((m) => !m.parent_id);
  const childMenus = menus.filter((m) => m.parent_id);
  const menuTree = topMenus.map((m) => ({ ...m, children: childMenus.filter((c) => c.parent_id === m.id) }));

  return (
    <div style={{ display: 'flex', minHeight: '100vh', background: 'var(--bg-parchment)' }}>
      <aside style={{
        width: collapsed ? 64 : 220, minHeight: '100vh', background: 'var(--bg-canvas)',
        borderRight: '1px solid var(--hairline)', transition: `width var(--duration-normal) var(--ease-out)`,
        display: 'flex', flexDirection: 'column', position: 'fixed', left: 0, top: 0, bottom: 0, zIndex: 100,
      }}>
        <div style={{ padding: '20px 16px', borderBottom: '1px solid var(--divider-soft)' }}>
          <h1 style={{ fontSize: collapsed ? 16 : 18, fontWeight: 600, color: 'var(--text-ink)', whiteSpace: 'nowrap', overflow: 'hidden' }}>
            {collapsed ? 'OM' : 'OpsMind'}
          </h1>
        </div>

        <nav style={{ flex: 1, padding: '8px 0', overflowY: 'auto' }}>
          {menuTree.map((m) => renderMenuItem(m))}
        </nav>

        <div style={{ padding: 12, borderTop: '1px solid var(--divider-soft)', display: 'flex', flexDirection: 'column', gap: 6 }}>
          <button onClick={() => router.push('/portal/messages')} style={sidebarBtn} aria-label={`消息${unreadCount > 0 ? ` ${unreadCount} 条未读` : ''}`}>
            <MessageSquare size={16} /> {!collapsed && <span>消息 {unreadCount > 0 && `(${unreadCount})`}</span>}
          </button>
          <button onClick={toggleTheme} style={sidebarBtn} aria-label={theme === 'dark' ? '切换浅色模式' : '切换暗色模式'}>
            {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
            {!collapsed && (theme === 'dark' ? '浅色模式' : '暗色模式')}
          </button>
        </div>
      </aside>

      <div style={{ flex: 1, marginLeft: collapsed ? 64 : 220, transition: `margin-left var(--duration-normal) var(--ease-out)`, display: 'flex', flexDirection: 'column' }}>
        <header style={{
          height: 52, background: 'var(--bg-canvas)', borderBottom: '1px solid var(--hairline)',
          display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 24px',
          position: 'sticky', top: 0, zIndex: 50, backdropFilter: 'saturate(180%) blur(20px)',
        }}>
          <button onClick={() => setCollapsed(!collapsed)} aria-label={collapsed ? '展开侧栏' : '折叠侧栏'} style={{ border: 'none', background: 'none', cursor: 'pointer', padding: 4, color: 'var(--text-ink)' }}>
            {collapsed ? <ChevronRight size={20} /> : <ChevronLeft size={20} />}
          </button>
          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
            <span style={{ fontSize: 13, color: 'var(--text-muted-48)' }}>{user?.real_name || user?.username}</span>
            <button onClick={() => { logout(); router.push('/login'); }} style={{ border: 'none', background: 'none', cursor: 'pointer', color: 'var(--text-muted-48)', display: 'flex', alignItems: 'center', gap: 4, fontSize: 13 }}>
              <LogOut size={14} /> 登出
            </button>
          </div>
        </header>
        <main style={{ flex: 1, padding: 24 }}>{children}</main>
      </div>
    </div>
  );
}

const sidebarBtn: React.CSSProperties = {
  display: 'flex', alignItems: 'center', gap: 10, padding: '8px 12px',
  border: 'none', background: 'transparent', color: 'var(--text-muted-80)',
  fontSize: 13, cursor: 'pointer', borderRadius: 'var(--radius-sm)',
};
