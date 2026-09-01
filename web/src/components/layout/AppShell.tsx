'use client';
// AppShell — 方向 C 统一 Shell：顶栏（折叠钮+品牌+主题+跨跳+账号）+ 可折叠侧栏（分区 nav）+ main。
// Portal/Admin 均映射 nav 到此组件，消除原双 shell 割裂。
import { useState, useEffect, useMemo, type ReactNode } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import Image from 'next/image';
import { useTheme } from '@/hooks/useTheme';
import { useConfigValue } from '@/hooks/useAppConfig';
import { SectionErrorBoundary } from '@/components/ErrorBoundary';
import { AccountSwitcher } from '@/components/shared/AccountSwitcher';
import { Button } from '@/components/ui/button';
import { GlobalCommand, type CommandGroupData } from '@/components/layout/GlobalCommand';
import { ChevronLeft, ChevronRight, ChevronDown, Sun, Moon } from 'lucide-react';

export interface NavItem {
  label: string;
  path: string;
  icon?: ReactNode;
  children?: NavItem[];
  badge?: ReactNode;
}

interface AppShellProps {
  nav: NavItem[];
  crossLink?: { label: string; path: string; icon: ReactNode };
  children: ReactNode;
}

const COLLAPSED = 68;
const EXPANDED = 240;

/** 顶层 active 逻辑：精确匹配或前缀（但若有更精确的 sibling 匹配则不激活，避免 /portal/tickets 误匹配 /portal/tickets/new） */
function isActive(itemPath: string, pathname: string, siblings: NavItem[]): boolean {
  if (pathname === itemPath) return true;
  if (!pathname.startsWith(itemPath + '/')) return false;
  const exactMatch = siblings.some((o) => o.path !== itemPath && o.path === pathname);
  return !exactMatch;
}

export function AppShell({ nav, crossLink, children }: AppShellProps) {
  const { theme, toggleTheme } = useTheme();
  const { value: appName } = useConfigValue('app_name');
  const pathname = usePathname();
  const router = useRouter();
  const [collapsed, setCollapsed] = useState(false);
  const [collapsedReady, setCollapsedReady] = useState(false);
  const [expandedMenus, setExpandedMenus] = useState<Set<string>>(new Set());

  useEffect(() => {
    setCollapsed(localStorage.getItem('sidebar-collapsed') === 'true');
    setCollapsedReady(true);
  }, []);
  useEffect(() => {
    if (collapsedReady) localStorage.setItem('sidebar-collapsed', String(collapsed));
  }, [collapsed, collapsedReady]);
  // 小屏自动折叠
  useEffect(() => {
    const mq = window.matchMedia('(max-width: 1023px)');
    const h = (e: MediaQueryListEvent) => { if (e.matches) setCollapsed(true); };
    if (mq.matches) setCollapsed(true);
    mq.addEventListener('change', h);
    return () => mq.removeEventListener('change', h);
  }, []);

  const toggleSubmenu = (id: string) =>
    setExpandedMenus((prev) => { const n = new Set(prev); n.has(id) ? n.delete(id) : n.add(id); return n; });

  const renderItem = (item: NavItem, depth = 0): ReactNode => {
    const hasChildren = !!item.children?.length;
    const active = depth === 0 ? isActive(item.path, pathname, nav) : item.path === pathname;
    const expanded = expandedMenus.has(item.path);
    const pad = collapsed ? '' : depth === 1 ? 'pl-[36px]' : depth === 2 ? 'pl-[52px]' : '';
    return (
      <div key={item.path}>
        <Button
          variant="menu"
          title={collapsed ? item.label : undefined}
          onClick={() => (hasChildren ? toggleSubmenu(item.path) : router.push(item.path))}
          className={`w-full justify-start ${collapsed ? 'justify-center' : ''} ${active ? '!bg-[var(--color-divider-soft)] font-semibold' : ''} ${pad}`}
          aria-current={active ? 'page' : undefined}
        >
          {item.icon}
          {!collapsed && (
            <span className="inline-flex items-center gap-3 flex-1">
              <span className="flex-1 text-left">{item.label}</span>
              {item.badge}
              {hasChildren && <ChevronDown size={16} className={`transition-transform duration-200 ${expanded ? 'rotate-180' : ''}`} />}
            </span>
          )}
        </Button>
        {!collapsed && hasChildren && expanded && item.children!.map((c) => renderItem(c, depth + 1))}
      </div>
    );
  };

  // ⌘K 命令面板内容：导航 + 快捷操作（从 nav + 主题/跨跳 自动构造）
  const commandGroups: CommandGroupData[] = useMemo(() => {
    const navItems = nav.flatMap((i) => [i, ...(i.children || [])]);
    const actions: { label: string; icon?: ReactNode; onSelect: () => void }[] = [
      { label: theme === 'dark' ? '切换浅色模式' : '切换暗色模式', onSelect: toggleTheme },
    ];
    if (crossLink) actions.push({ label: crossLink.label, icon: crossLink.icon, onSelect: () => router.push(crossLink.path) });
    return [
      { heading: '导航', items: navItems.map((i) => ({ label: i.label, icon: i.icon, onSelect: () => router.push(i.path) })) },
      { heading: '快捷操作', items: actions },
    ];
  }, [nav, theme, toggleTheme, crossLink, router]);

  const sidebarWidth = collapsed ? COLLAPSED : EXPANDED;

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
          {nav.map((item) => renderItem(item))}
        </nav>
      </aside>

      <div className="flex-1 flex flex-col transition-[margin-left] duration-[250ms]" style={{ marginLeft: sidebarWidth }}>
        <header className="h-[var(--header-height)] flex items-center justify-between px-6 bg-[var(--color-canvas)]/80 border-b border-[var(--color-hairline)] sticky top-0 z-[var(--z-nav)] backdrop-blur-xl">
          <Button variant="menu" size="icon" onClick={() => setCollapsed(!collapsed)} aria-label={collapsed ? '展开侧栏' : '折叠侧栏'}>
            {collapsed ? <ChevronRight /> : <ChevronLeft />}
          </Button>
          <div className="flex items-center gap-3">
            {crossLink && (
              <Button variant="menu" size="icon" onClick={() => router.push(crossLink.path)} aria-label={crossLink.label}>
                {crossLink.icon}
              </Button>
            )}
            <Button variant="menu" size="icon" onClick={toggleTheme} aria-label={theme === 'dark' ? '切换浅色模式' : '切换暗色模式'}>
              {theme === 'dark' ? <Sun /> : <Moon />}
            </Button>
            {collapsedReady && <AccountSwitcher />}
          </div>
        </header>
        <main className="flex-1 p-6 max-w-wide w-full mx-auto">
          <SectionErrorBoundary>{children}</SectionErrorBoundary>
        </main>
      </div>

      <GlobalCommand groups={commandGroups} />
    </div>
  );
}
