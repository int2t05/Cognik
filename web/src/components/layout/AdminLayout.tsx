/** AdminLayout — 后台管理布局（侧栏嵌套菜单 + 顶栏 + 内容区）。 */

'use client';

import { useState, useEffect, useMemo } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { useTheme } from '@/hooks/useTheme';
import { getUnreadCount } from '@/lib/api/message';
import { isActivePath } from '@/lib/menu';
import { AppleButton } from '@/components/ui/AppleButton';
import { SectionErrorBoundary } from '@/components/ErrorBoundary';
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

  const depthPadding = (depth: number): string => {
    if (depth === 1) return 'pl-[36px]';
    if (depth === 2) return 'pl-[52px]';
    return '';
  };

  const renderMenuItem = (m: MenuItem, depth = 0) => {
    const hasChildren = m.children && m.children.length > 0;
    const active = isActivePath(m.path, pathname);
    const expanded = expandedMenus.has(m.id);

    const btnClass = [
      'flex items-center gap-3 w-full px-5 py-2.5 border-0 bg-transparent text-[var(--color-ink)] text-sm cursor-pointer text-left transition hover:bg-[var(--color-divider-soft)]',
      collapsed ? 'justify-center px-0 py-3' : '',
      active ? 'bg-[var(--color-divider-soft)] text-[var(--color-accent)] font-semibold' : '',
      depthPadding(depth),
    ].filter(Boolean).join(' ');

    return (
      <div key={m.id}>
        <button
          onClick={() => { if (hasChildren) toggleSubmenu(m.id); else router.push(m.path); }}
          title={collapsed ? m.name : undefined}
          className={btnClass}
        >
          {ICON_MAP[m.icon] || <Settings size={18} />}
          {!collapsed && <span className="flex-1">{m.name}</span>}
          {!collapsed && hasChildren && (
            <ChevronDown size={14} className={`transition-transform duration-200 ${expanded ? 'rotate-180' : ''}`} />
          )}
        </button>
        {!collapsed && hasChildren && expanded && m.children!.map((c) => renderMenuItem(c, depth + 1))}
      </div>
    );
  };

  const menuTree = useMemo(() => {
    const topMenus = menus.filter((m) => !m.parent_id);
    const childMenus = menus.filter((m) => m.parent_id);
    return topMenus.map((m) => ({ ...m, children: childMenus.filter((c) => c.parent_id === m.id) }));
  }, [menus]);

  const sidebarWidth = collapsed ? 64 : 220;

  return (
    <div className="flex min-h-screen bg-[var(--color-parchment)]">
      <aside
        className="flex flex-col fixed left-0 top-0 bottom-0 z-[100] bg-[var(--color-canvas)] border-r border-[var(--color-hairline)] transition-[width] duration-250 ease-[cubic-bezier(0.16,1,0.3,1)]"
        style={{ width: sidebarWidth }}
      >
        <div className={`px-4 py-5 border-b border-[var(--color-divider-soft)] whitespace-nowrap overflow-hidden ${collapsed ? 'text-base' : 'text-lg font-semibold text-[var(--color-ink)]'}`}>
          {collapsed ? 'OM' : 'OpsMind'}
        </div>

        <nav className="flex-1 py-2 overflow-y-auto">
          {menuTree.map((m) => renderMenuItem(m))}
        </nav>

        <div className="p-3 border-t border-[var(--color-divider-soft)] flex flex-col gap-1.5">
          <button onClick={() => router.push('/portal/messages')} className="flex items-center gap-2.5 px-3 py-2 border-0 bg-transparent text-[var(--color-text-muted-80)] text-[13px] cursor-pointer rounded-lg transition hover:bg-[var(--color-divider-soft)]" aria-label={`消息${unreadCount > 0 ? ` ${unreadCount} 条未读` : ''}`}>
            <MessageSquare size={16} /> {!collapsed && <span>消息 {unreadCount > 0 && `(${unreadCount})`}</span>}
          </button>
          <button onClick={toggleTheme} className="flex items-center gap-2.5 px-3 py-2 border-0 bg-transparent text-[var(--color-text-muted-80)] text-[13px] cursor-pointer rounded-lg transition hover:bg-[var(--color-divider-soft)]" aria-label={theme === 'dark' ? '切换浅色模式' : '切换暗色模式'}>
            {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
            {!collapsed && (theme === 'dark' ? '浅色模式' : '暗色模式')}
          </button>
        </div>
      </aside>

      <div className="flex-1 flex flex-col transition-[margin-left] duration-250" style={{ marginLeft: sidebarWidth }}>
        <header className="h-[52px] flex items-center justify-between px-6 bg-[var(--color-canvas)] border-b border-[var(--color-hairline)] sticky top-0 z-50 backdrop-blur-[20px] backdrop-saturate-[180%]">
          <button onClick={() => setCollapsed(!collapsed)} aria-label={collapsed ? '展开侧栏' : '折叠侧栏'} className="border-0 bg-transparent cursor-pointer p-1 text-[var(--color-ink)]">
            {collapsed ? <ChevronRight size={20} /> : <ChevronLeft size={20} />}
          </button>
          <div className="flex items-center gap-4">
            <span className="text-[13px] text-[var(--color-text-muted-48)]">{user?.real_name || user?.username}</span>
            <button onClick={() => { logout(); router.push('/login'); }} className="flex items-center gap-1 border-0 bg-transparent cursor-pointer text-[var(--color-text-muted-48)] text-[13px]">
              <LogOut size={14} /> 登出
            </button>
          </div>
        </header>
        <main className="flex-1 p-6"><SectionErrorBoundary>{children}</SectionErrorBoundary></main>
      </div>
    </div>
  );
}
