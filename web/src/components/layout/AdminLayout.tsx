/** AdminLayout — 后台管理布局（侧栏嵌套菜单 + 顶栏 + 内容区）。 */

'use client';

import { useState, useEffect, useMemo } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import Image from 'next/image';
import { useAuth } from '@/hooks/useAuth';
import { useTheme } from '@/hooks/useTheme';
import { useUnreadCount } from '@/hooks/useUnreadCount';
import { useConfigValue } from '@/hooks/useAppConfig';
import { isActivePath } from '@/lib/menu';
import { SectionErrorBoundary } from '@/components/ErrorBoundary';
import { AccountSwitcher } from '@/components/shared/AccountSwitcher';
import { LayoutDashboard, Ticket, BookOpen, Users, Shield, Settings, ScrollText, MessageSquare, ChevronLeft, ChevronRight, Sun, Moon, ChevronDown, Cpu, FileText, User, Bot } from 'lucide-react';

// ICON_MAP 将后端菜单 icon 字段映射到 Lucide 图标组件。
// 同时兼容旧值（如 knowledge → BookOpen），确保后端数据变动时不挂。
const ICON_MAP: Record<string, React.ReactNode> = {
  dashboard: <LayoutDashboard size={16} />,
  ticket: <Ticket size={16} />,
  knowledge: <BookOpen size={16} />,
  book: <BookOpen size={16} />,
  users: <Users size={16} />,
  user: <User size={16} />,
  role: <Shield size={16} />,
  shield: <Shield size={16} />,
  config: <Settings size={16} />,
  settings: <Settings size={16} />,
  audit: <ScrollText size={16} />,
  'file-text': <FileText size={16} />,
  message: <MessageSquare size={16} />,
  cpu: <Cpu size={16} />,
};

// FRONTEND_ROUTES 将后端菜单路径映射到实际前端路由。
const FRONTEND_ROUTES: Record<string, string> = {
  '/admin/audit-logs': '/admin/audit',
};

const SIDEBAR_COLLAPSED_WIDTH = 64;
const SIDEBAR_EXPANDED_WIDTH = 240;

interface MenuItem { id: number; name: string; path: string; icon: string; parent_id: number; sort_order: number; type: string; children?: MenuItem[]; }

export function AdminLayout({ children }: { children: React.ReactNode }) {
  const { user, menus } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const { value: appName } = useConfigValue('app_name');
  const pathname = usePathname();
  const router = useRouter();
  const [collapsed, setCollapsed] = useState(false);
  const [collapsedReady, setCollapsedReady] = useState(false);
  const [mounted, setMounted] = useState(false);
  const [expandedMenus, setExpandedMenus] = useState<Set<number>>(new Set());
  const { unreadCount } = useUnreadCount();

  // 客户端挂载后才能安全读取 localStorage 和 useAuth 返回的菜单数据，
  // 避免 SSR 与客户端 hydration 不一致（服务端 menus=[]，客户端有实际数据）
  useEffect(() => {
    setMounted(true);
    const saved = localStorage.getItem('sidebar-collapsed') === 'true';
    setCollapsed(saved);
    setCollapsedReady(true);
  }, []);

  useEffect(() => {
    if (collapsedReady) localStorage.setItem('sidebar-collapsed', String(collapsed));
  }, [collapsed, collapsedReady]);

  const toggleSubmenu = (id: number) => {
    setExpandedMenus((prev) => { const next = new Set(prev); if (next.has(id)) next.delete(id); else next.add(id); return next; });
  };

  const depthPadding = (depth: number): string => {
    if (collapsed) return '';
    if (depth === 1) return 'pl-[36px]';
    if (depth === 2) return 'pl-[52px]';
    return '';
  };

  const renderMenuItem = (m: MenuItem, depth = 0) => {
    const hasChildren = m.children && m.children.length > 0;
    const targetPath = FRONTEND_ROUTES[m.path] || m.path;
    const active = isActivePath(targetPath, pathname);
    const expanded = expandedMenus.has(m.id);

    const btnClass = [
      'flex items-center gap-3 w-full px-5 py-2.5 min-h-[44px] border-0 bg-transparent text-[var(--color-ink)] text-caption cursor-pointer text-left rounded-[var(--radius-pill)] transition active:scale-95 hover:bg-[var(--color-divider-soft)]',
      collapsed ? 'justify-center px-0 py-3' : '',
      active ? 'bg-[var(--color-divider-soft)] text-[var(--color-ink)] font-semibold shadow-[inset_4px_0_0_var(--color-accent)]' : '',
      depthPadding(depth),
    ].filter(Boolean).join(' ');

    return (
      <div key={m.id}>
        <button
          onClick={() => { if (hasChildren) toggleSubmenu(m.id); else router.push(targetPath); }}
          title={collapsed ? m.name : undefined}
          className={btnClass}
          aria-current={active ? 'page' : undefined}
        >
          {ICON_MAP[m.icon] || <Settings size={16} />}
          {!collapsed && <span className="flex-1">{m.name}</span>}
          {!collapsed && hasChildren && (
            <ChevronDown size={16} className={`transition-transform duration-200 ${expanded ? 'rotate-180' : ''}`} />
          )}
        </button>
        {!collapsed && hasChildren && expanded && m.children!.map((c) => renderMenuItem(c, depth + 1))}
      </div>
    );
  };

  const menuTree = useMemo(() => {
    const topMenus = menus.filter((m) => !m.parent_id);
    const childMenus = menus.filter((m) => m.parent_id);
    // 去重：多条菜单项可能映射到同一前端路由，按 sort_order 保留第一条
    const seenRoutes = new Set<string>();
    const deduped = topMenus.filter((m) => {
      const route = FRONTEND_ROUTES[m.path] || m.path;
      if (seenRoutes.has(route)) return false;
      seenRoutes.add(route);
      return true;
    });
    return deduped.map((m) => ({ ...m, children: childMenus.filter((c) => c.parent_id === m.id) }));
  }, [menus]);

  // 小屏（< 1024px）自动折叠侧栏，避免手动操作
  useEffect(() => {
    const mq = window.matchMedia('(max-width: 1023px)');
    const handler = (e: MediaQueryListEvent) => { if (e.matches) setCollapsed(true); };
    if (mq.matches) setCollapsed(true);
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  }, []);

  const sidebarWidth = collapsed ? SIDEBAR_COLLAPSED_WIDTH : SIDEBAR_EXPANDED_WIDTH;

  return (
    <div className="flex min-h-screen bg-[var(--color-parchment)]">
      <aside
        className="flex flex-col fixed left-0 top-0 bottom-0 z-[var(--z-nav)] bg-[var(--color-canvas)] border-r border-[var(--color-hairline)] shadow-[var(--shadow-sidebar)] transition-[width] duration-[250ms] ease-[cubic-bezier(0.16,1,0.3,1)]"
        style={{ width: sidebarWidth }}
      >
        <div className={`flex items-center gap-3 px-4 py-4 border-b border-[var(--color-divider-soft)] overflow-hidden ${collapsed ? 'justify-center' : ''}`}>
          <Image src="/icon.svg" alt="" width={28} height={28} className="shrink-0" />
          {!collapsed && <span className="text-headline font-semibold text-[var(--color-ink)] truncate">{appName || 'OpsMind'}</span>}
        </div>

        <nav className="flex-1 py-2 overflow-y-auto">
          {mounted ? menuTree.map((m) => renderMenuItem(m)) : (
            <div className="flex justify-center py-6">
              <div className="w-5 h-5 border-2 border-[var(--color-divider-soft)] border-t-[var(--color-accent)] rounded-full animate-spin" />
            </div>
          )}
        </nav>

        <div className="p-3 border-t border-[var(--color-divider-soft)] flex flex-col gap-1.5">
          <button onClick={() => router.push('/portal/chat')} className="flex items-center gap-2.5 px-3 py-3 min-h-[44px] border-0 bg-transparent text-[var(--color-text-muted-80)] text-caption cursor-pointer rounded-[var(--radius-pill)] transition hover:bg-[var(--color-divider-soft)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent-focus)]" aria-label="门户首页">
            <Bot size={16} /> {!collapsed && <span>门户</span>}
          </button>
          <button onClick={() => router.push('/portal/messages')} className="flex items-center gap-2.5 px-3 py-3 min-h-[44px] border-0 bg-transparent text-[var(--color-text-muted-80)] text-caption cursor-pointer rounded-[var(--radius-pill)] transition hover:bg-[var(--color-divider-soft)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent-focus)]" aria-label={`消息${unreadCount > 0 ? ` ${unreadCount} 条未读` : ''}`}>
            <MessageSquare size={16} /> {!collapsed && <span>消息 {unreadCount > 0 && `(${unreadCount})`}</span>}
          </button>
          <button onClick={toggleTheme} className="flex items-center gap-2.5 px-3 py-3 min-h-[44px] border-0 bg-transparent text-[var(--color-text-muted-80)] text-caption cursor-pointer rounded-[var(--radius-pill)] transition hover:bg-[var(--color-divider-soft)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent-focus)]" aria-label={theme === 'dark' ? '切换浅色模式' : '切换暗色模式'}>
            {theme === 'dark' ? <Sun size={16} /> : <Moon size={16} />}
            {!collapsed && (theme === 'dark' ? '浅色模式' : '暗色模式')}
          </button>
        </div>
      </aside>

      <div className="flex-1 flex flex-col transition-[margin-left] duration-[250ms]" style={{ marginLeft: sidebarWidth }}>
        <header className="h-[var(--header-height)] flex items-center justify-between px-6 bg-[var(--color-canvas)]/80 border-b border-[var(--color-hairline)] sticky top-0 z-[var(--z-nav)] backdrop-blur-xl">
          <button onClick={() => setCollapsed(!collapsed)} aria-label={collapsed ? '展开侧栏' : '折叠侧栏'} className="border-0 bg-transparent cursor-pointer p-3 text-[var(--color-ink)] transition hover:opacity-70 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent-focus)]">
            {collapsed ? <ChevronRight size={20} /> : <ChevronLeft size={20} />}
          </button>
          <div className="flex items-center gap-[var(--spacing-md-plus)]">
            {mounted && <span className="text-caption text-[var(--color-text-muted-48)]">{user?.real_name || user?.username}</span>}
            {mounted && <AccountSwitcher />}
          </div>
        </header>
        <main className="flex-1 p-6 max-w-wide w-full mx-auto"><SectionErrorBoundary>{children}</SectionErrorBoundary></main>
      </div>
    </div>
  );
}
