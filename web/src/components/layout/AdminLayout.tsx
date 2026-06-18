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
import styles from './AdminLayout.module.css';

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

  const depthClass = (depth: number): string => {
    if (depth === 1) return styles.menuItemDepth1;
    if (depth === 2) return styles.menuItemDepth2;
    return '';
  };

  const renderMenuItem = (m: MenuItem, depth = 0) => {
    const hasChildren = m.children && m.children.length > 0;
    const active = isActivePath(m.path, pathname);
    const expanded = expandedMenus.has(m.id);

    const btnClass = [
      styles.menuItem,
      collapsed ? styles.menuItemCollapsed : '',
      active ? styles.menuItemActive : '',
      depthClass(depth),
    ].filter(Boolean).join(' ');

    return (
      <div key={m.id}>
        <button
          onClick={() => { if (hasChildren) toggleSubmenu(m.id); else router.push(m.path); }}
          title={collapsed ? m.name : undefined}
          className={btnClass}
        >
          {ICON_MAP[m.icon] || <Settings size={18} />}
          {!collapsed && <span className={styles.menuLabel}>{m.name}</span>}
          {!collapsed && hasChildren && (
            <ChevronDown size={14} className={`${styles.menuChevron} ${expanded ? styles.menuChevronOpen : ''}`} />
          )}
        </button>
        {!collapsed && hasChildren && expanded && m.children!.map((c) => renderMenuItem(c, depth + 1))}
      </div>
    );
  };

  const topMenus = menus.filter((m) => !m.parent_id);
  const childMenus = menus.filter((m) => m.parent_id);
  const menuTree = topMenus.map((m) => ({ ...m, children: childMenus.filter((c) => c.parent_id === m.id) }));

  return (
    <div className={styles.layout}>
      <aside className={`${styles.sidebar} ${collapsed ? styles.sidebarCollapsed : ''}`}>
        <div className={`${styles.logo} ${collapsed ? styles.logoCollapsed : ''}`}>
          {collapsed ? 'OM' : 'OpsMind'}
        </div>

        <nav className={styles.nav}>
          {menuTree.map((m) => renderMenuItem(m))}
        </nav>

        <div className={styles.bottomActions}>
          <button onClick={() => router.push('/portal/messages')} className={styles.actionBtn} aria-label={`消息${unreadCount > 0 ? ` ${unreadCount} 条未读` : ''}`}>
            <MessageSquare size={16} /> {!collapsed && <span>消息 {unreadCount > 0 && `(${unreadCount})`}</span>}
          </button>
          <button onClick={toggleTheme} className={styles.actionBtn} aria-label={theme === 'dark' ? '切换浅色模式' : '切换暗色模式'}>
            {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
            {!collapsed && (theme === 'dark' ? '浅色模式' : '暗色模式')}
          </button>
        </div>
      </aside>

      <div className={`${styles.main} ${collapsed ? styles.mainCollapsed : styles.mainExpanded}`}>
        <header className={styles.topbar}>
          <button onClick={() => setCollapsed(!collapsed)} aria-label={collapsed ? '展开侧栏' : '折叠侧栏'} className={styles.toggleBtn}>
            {collapsed ? <ChevronRight size={20} /> : <ChevronLeft size={20} />}
          </button>
          <div className={styles.userInfo}>
            <span className={styles.userName}>{user?.real_name || user?.username}</span>
            <button onClick={() => { logout(); router.push('/login'); }} className={styles.logoutBtn}>
              <LogOut size={14} /> 登出
            </button>
          </div>
        </header>
        <main className={styles.content}>{children}</main>
      </div>
    </div>
  );
}
