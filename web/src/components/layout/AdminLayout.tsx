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
import { AppleButton } from '@/components/ui/AppleButton';
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
// 同时兼容旧版本种子数据中的错误路径，避免升级后菜单 404。
const FRONTEND_ROUTES: Record<string, string> = {
  '/admin/audit-logs': '/admin/audit',
  // 旧版菜单路径兼容（commit 16bd0ab 及更早的 seed 数据）
  '/admin/model-config': '/admin/config/llm',
  '/admin/llm-config': '/admin/config/llm',
  '/admin/system-config': '/admin/config/system',
};

const SIDEBAR_COLLAPSED_WIDTH = 68;
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
      'w-full justify-start',
      collapsed ? 'justify-center' : '',
      active ? '!bg-[var(--color-divider-soft)] font-semibold' : '',
      depthPadding(depth),
    ].filter(Boolean).join(' ');

    return (
      <div key={m.id}>
        <AppleButton
          variant="menu"
          icon={ICON_MAP[m.icon] || <Settings />}
          onClick={() => { if (hasChildren) toggleSubmenu(m.id); else router.push(targetPath); }}
          title={collapsed ? m.name : undefined}
          className={btnClass}
          aria-current={active ? 'page' : undefined}
        >
          {!collapsed && (
            <span className="inline-flex items-center gap-3 flex-1">
              <span className="flex-1 text-left">{m.name}</span>
              {hasChildren && <ChevronDown size={16} className={`transition-transform duration-200 ${expanded ? 'rotate-180' : ''}`} />}
            </span>
          )}
        </AppleButton>
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
        className="flex flex-col fixed left-0 top-0 bottom-0 z-[var(--z-nav)] bg-[var(--color-canvas)] border-r border-[var(--color-hairline)] transition-[width] duration-[250ms] ease-[cubic-bezier(0.16,1,0.3,1)]"
        style={{ width: sidebarWidth }}
      >
        <div className={`flex items-center gap-3 px-4 py-4 border-b border-[var(--color-divider-soft)] overflow-hidden ${collapsed ? 'justify-center' : ''}`}>
          <Image src="/icon.svg" alt="" width={28} height={28} className="shrink-0" />
          {!collapsed && <span className="text-title font-semibold text-[var(--color-ink)] truncate">{appName || 'OpsMind'}</span>}
        </div>

        <nav className="flex-1 py-2 overflow-y-auto overscroll-behavior-contain" aria-label="主导航">
          {mounted ? menuTree.map((m) => renderMenuItem(m)) : (
            <div className="flex justify-center py-6">
              <div className="w-5 h-5 border-2 border-[var(--color-divider-soft)] border-t-[var(--color-accent)] rounded-full animate-spin" />
            </div>
          )}
        </nav>

        <div className="border-t border-[var(--color-divider-soft)]">
          <div className="relative">
            <AppleButton variant="menu" icon={<MessageSquare />} onClick={() => router.push('/portal/messages')}
              aria-label={`消息${unreadCount > 0 ? ` ${unreadCount} 条未读` : ''}`}
              aria-current={isActivePath('/portal/messages', pathname) ? 'page' : undefined}
              className={`w-full justify-start ${collapsed ? 'justify-center' : ''} ${isActivePath('/portal/messages', pathname) ? '!bg-[var(--color-divider-soft)] font-semibold' : ''}`}>
              {!collapsed && (
                <span className="inline-flex items-center gap-3 flex-1">
                  <span className="flex-1 text-left">消息</span>
                </span>
              )}
            </AppleButton>
            {unreadCount > 0 && (
              <span className={`absolute bg-[var(--color-error)] text-[var(--color-canvas)] rounded-full flex items-center justify-center ${collapsed ? '-top-0.5 -right-0.5 w-3 h-3' : 'top-1 right-2 min-w-[14px] h-3.5 px-1 text-[10px] leading-none'}`}>
                {collapsed ? '' : unreadCount > 99 ? '99+' : unreadCount}
              </span>
            )}
          </div>
        </div>

      </aside>

      <div className="flex-1 flex flex-col transition-[margin-left] duration-[250ms]" style={{ marginLeft: sidebarWidth }}>
        <header className="h-[var(--header-height)] flex items-center justify-between px-6 bg-[var(--color-canvas)]/80 border-b border-[var(--color-hairline)] sticky top-0 z-[var(--z-nav)] backdrop-blur-xl">
          <AppleButton variant="menu" icon={collapsed ? <ChevronRight /> : <ChevronLeft />}
            onClick={() => setCollapsed(!collapsed)}
            aria-label={collapsed ? '展开侧栏' : '折叠侧栏'} />
          <div className="flex items-center gap-3">
            <AppleButton variant="menu" icon={theme === 'dark' ? <Sun /> : <Moon />} onClick={toggleTheme}
              aria-label={theme === 'dark' ? '切换浅色模式' : '切换暗色模式'} />
            <AppleButton variant="menu" icon={<Bot />} onClick={() => router.push('/portal/chat')} aria-label="门户首页" />
            {mounted && <span className="text-caption text-[var(--color-text-muted-48)] mr-1">{user?.real_name || user?.username}</span>}
            {mounted && <AccountSwitcher />}
          </div>
        </header>
        <main className="flex-1 p-6 max-w-wide w-full mx-auto"><SectionErrorBoundary>{children}</SectionErrorBoundary></main>
      </div>
    </div>
  );
}
