'use client';
// AppShell — 统一 Shell：顶栏（品牌 + 内联全局搜索 + 主题 + 账号）+ 可折叠侧栏（分区 nav）+ main。
// 搜索框内联下拉即时结果（导航项 + 快捷操作过滤），非独立弹窗。
// Portal/Admin 共用：管理员分区由调用方通过 nav 传入（权限判断在 layout 层）。
import { useState, useEffect, useMemo, useRef, useCallback, useSyncExternalStore, type ReactNode } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import Image from 'next/image';
import { useTheme } from '@/hooks/useTheme';
import { useConfigValue } from '@/hooks/useAppConfig';
import { SectionErrorBoundary } from '@/components/ErrorBoundary';
import { AccountSwitcher } from '@/components/shared/AccountSwitcher';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Separator } from '@/components/ui/separator';
import { ChevronLeft, ChevronRight, ChevronDown, Sun, Moon, Search } from 'lucide-react';

export interface NavItem {
  label: string;
  path: string;
  icon?: ReactNode;
  children?: NavItem[];
  badge?: ReactNode;
}

export interface NavSection {
  heading?: string;
  items: NavItem[];
}

interface AppShellProps {
  nav: NavSection[];
  crossLink?: { label: string; path: string; icon: ReactNode };
  /** 隐藏侧栏（Chat 聚焦模式） */
  hideSidebar?: boolean;
  /** 顶栏与内容之间的子栏（面包屑/操作） */
  subbar?: ReactNode;
  /** 内容是否需要内边距（页面自带布局时传 false） */
  padded?: boolean;
  children: ReactNode;
}

const COLLAPSED = 68;
const EXPANDED = 220;

/** 顶层 active 逻辑：精确匹配或前缀（但若有更精确的 sibling 匹配则不激活） */
function isActive(itemPath: string, pathname: string, siblings: NavItem[]): boolean {
  if (pathname === itemPath) return true;
  if (!pathname.startsWith(itemPath + '/')) return false;
  const exactMatch = siblings.some((o) => o.path !== itemPath && o.path === pathname);
  return !exactMatch;
}

/** 侧栏折叠状态：useSyncExternalStore 读写 localStorage，SSR 安全。 */
function useSidebarCollapsed(): [boolean, (value: boolean) => void] {
  const collapsed = useSyncExternalStore(
    (cb) => {
      window.addEventListener('sidebar-collapsed-change', cb);
      window.addEventListener('storage', cb);
      return () => {
        window.removeEventListener('sidebar-collapsed-change', cb);
        window.removeEventListener('storage', cb);
      };
    },
    () => {
      const pref = localStorage.getItem('sidebar-collapsed');
      if (pref === 'true') return true;
      if (pref === 'false') return false;
      // 无偏好时移动端自动折叠
      return window.matchMedia('(max-width: 1023px)').matches;
    },
    () => false,
  );
  const setCollapsed = useCallback((value: boolean) => {
    localStorage.setItem('sidebar-collapsed', String(value));
    window.dispatchEvent(new Event('sidebar-collapsed-change'));
  }, []);
  return [collapsed, setCollapsed];
}

/** 客户端就绪检测：SSR 时 false，hydration 后 true。 */
function useIsClient(): boolean {
  return useSyncExternalStore(() => () => {}, () => true, () => false);
}

export function AppShell({ nav, crossLink, hideSidebar = false, subbar, padded = true, children }: AppShellProps) {
  const { theme, toggleTheme } = useTheme();
  const { value: appName } = useConfigValue('app_name');
  const pathname = usePathname();
  const router = useRouter();
  const [collapsed, setCollapsed] = useSidebarCollapsed();
  const [expandedMenus, setExpandedMenus] = useState<Set<string>>(new Set());
  const isClient = useIsClient();

  // 移动端 viewport 变化时自动折叠（初始检查由 getSnapshot 处理，effect 仅注册监听）
  useEffect(() => {
    const mq = window.matchMedia('(max-width: 1023px)');
    const handler = (e: MediaQueryListEvent) => { if (e.matches) setCollapsed(true); };
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  }, [setCollapsed]);

  const toggleSubmenu = (id: string) =>
    setExpandedMenus((prev) => { const n = new Set(prev); n.has(id) ? n.delete(id) : n.add(id); return n; });

  const renderItem = (item: NavItem, depth = 0, siblings: NavItem[]): ReactNode => {
    const hasChildren = !!item.children?.length;
    const active = depth === 0 ? isActive(item.path, pathname, siblings) : item.path === pathname;
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
        {!collapsed && hasChildren && expanded && item.children!.map((c) => renderItem(c, depth + 1, item.children!))}
      </div>
    );
  };

  const sidebarVisible = !hideSidebar;
  const sidebarWidth = sidebarVisible ? (collapsed ? COLLAPSED : EXPANDED) : 0;

  return (
    <div className="flex min-h-screen bg-[var(--color-parchment)]">
      {sidebarVisible && (
        <aside
          className="flex flex-col fixed left-0 top-0 bottom-0 z-[var(--z-nav)] bg-[var(--color-canvas)] border-r border-[var(--color-hairline)] transition-[width] duration-[250ms] ease-[cubic-bezier(0.16,1,0.3,1)]"
          style={{ width: sidebarWidth }}
        >
          <div className={`flex items-center gap-3 px-4 py-4 border-b border-[var(--color-divider-soft)] overflow-hidden ${collapsed ? 'justify-center' : ''}`}>
            <Image src="/icon.svg" alt="" width={28} height={28} className="shrink-0" />
            {!collapsed && <span className="text-title font-semibold text-[var(--color-ink)] truncate">{appName || 'OpsMind'}</span>}
          </div>
          <nav className="flex-1 py-2 overflow-y-auto overscroll-behavior-contain" aria-label="主导航">
            {nav.map((section, idx) => (
              <div key={section.heading || `s${idx}`}>
                {!collapsed && section.heading && (
                  <div className="px-3 py-1 text-fine text-[var(--color-text-muted-48)] uppercase tracking-wide">{section.heading}</div>
                )}
                {section.items.map((item) => renderItem(item, 0, section.items))}
                {idx < nav.length - 1 && <div className="px-3 my-2"><Separator /></div>}
              </div>
            ))}
          </nav>
        </aside>
      )}

      <div className="flex-1 flex flex-col transition-[margin-left] duration-[250ms]" style={{ marginLeft: sidebarWidth }}>
        <header className="h-[var(--header-height)] flex items-center gap-4 px-5 bg-[var(--color-canvas)]/80 border-b border-[var(--color-hairline)] sticky top-0 z-[var(--z-nav)] backdrop-blur-xl">
          {sidebarVisible && (
            <Button variant="menu" size="icon" onClick={() => setCollapsed(!collapsed)} aria-label={collapsed ? '展开侧栏' : '折叠侧栏'}>
              {collapsed ? <ChevronRight /> : <ChevronLeft />}
            </Button>
          )}
          <span className="text-callout font-semibold text-[var(--color-ink)] shrink-0">{appName || 'OpsMind'}</span>
          <GlobalSearch nav={nav} crossLink={crossLink} />
          <div className="flex-1" />
          {crossLink && (
            <Button variant="menu" size="icon" onClick={() => router.push(crossLink.path)} aria-label={crossLink.label}>
              {crossLink.icon}
            </Button>
          )}
          <Button variant="menu" size="icon" onClick={toggleTheme} aria-label={theme === 'dark' ? '切换浅色模式' : '切换暗色模式'}>
            {theme === 'dark' ? <Sun /> : <Moon />}
          </Button>
          {isClient && <AccountSwitcher />}
        </header>
        {subbar && (
          <div className="h-[var(--header-height)] flex items-center gap-3 px-6 bg-[var(--color-canvas)] border-b border-[var(--color-hairline)]">
            {subbar}
          </div>
        )}
        <main className={`flex-1 min-h-0 w-full overflow-hidden ${padded ? 'max-w-wide mx-auto' : ''}`}>
          <SectionErrorBoundary>{children}</SectionErrorBoundary>
        </main>
      </div>
    </div>
  );
}

/** GlobalSearch — 顶栏内联搜索。输入即时过滤导航项 + 快捷操作，下拉展示结果。 */
function GlobalSearch({ nav, crossLink }: { nav: NavSection[]; crossLink?: { label: string; path: string; icon: ReactNode } }) {
  const router = useRouter();
  const [query, setQuery] = useState('');
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  // ⌘K 聚焦搜索框
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        ref.current?.querySelector('input')?.focus();
      }
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, []);

  // 点击外部关闭下拉
  useEffect(() => {
    if (!open) return;
    const h = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false); };
    document.addEventListener('mousedown', h);
    return () => document.removeEventListener('mousedown', h);
  }, [open]);

  const results = useMemo(() => {
    if (!query.trim()) return { nav: [], actions: [] };
    const q = query.toLowerCase();
    // 扁平化所有导航项（含子菜单），附带分区名
    const navItems: (NavItem & { section?: string })[] = [];
    for (const s of nav) {
      for (const i of s.items) {
        navItems.push({ ...i, section: s.heading });
        for (const c of i.children || []) navItems.push({ ...c, section: s.heading });
      }
    }
    const matchedNav = navItems.filter((i) => i.label.toLowerCase().includes(q)).slice(0, 6);
    const matchedActions: { label: string; icon?: ReactNode; path?: string }[] = [];
    if ('切换主题'.includes(q) || 'theme'.includes(q)) matchedActions.push({ label: '切换主题', icon: <Moon size={14} /> });
    if (crossLink && crossLink.label.toLowerCase().includes(q)) matchedActions.push({ label: crossLink.label, icon: crossLink.icon, path: crossLink.path });
    return { nav: matchedNav, actions: matchedActions };
  }, [query, nav, crossLink]);

  const hasResults = results.nav.length > 0 || results.actions.length > 0;

  return (
    <div ref={ref} className="relative w-full max-w-[360px]">
      <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--color-text-muted-48)] pointer-events-none z-10" />
      <Input
        value={query}
        onChange={(e) => { setQuery(e.target.value); setOpen(true); }}
        onFocus={() => setOpen(true)}
        placeholder="搜索工单、知识、会话…"
        className="h-8 text-fine pl-9 pr-12 rounded-[var(--radius-md)] bg-[var(--color-tile-1)] border-[var(--color-hairline)]"
        aria-label="全局搜索"
      />
      <kbd className="absolute right-2.5 top-1/2 -translate-y-1/2 font-[var(--font-mono)] text-fine bg-[var(--color-canvas)] border border-[var(--color-hairline)] rounded px-1.5 py-0.5 text-[var(--color-text-muted-48)] pointer-events-none">⌘K</kbd>
      {open && query.trim() && (
        <div className="absolute top-full left-0 right-0 mt-1 bg-[var(--color-canvas)] rounded-[var(--radius-md)] border border-[var(--color-hairline)] shadow-[var(--shadow-dialog)] z-[var(--z-overlay)] overflow-hidden">
          {!hasResults ? (
            <div className="px-4 py-6 text-center text-fine text-[var(--color-text-muted-48)]">无匹配结果</div>
          ) : (
            <>
              {results.nav.length > 0 && (
                <div className="py-1">
                  <div className="px-3 py-1 text-fine text-[var(--color-text-muted-48)] uppercase tracking-wide">导航</div>
                  {results.nav.map((r) => (
                    <button
                      key={r.path}
                      onClick={() => { router.push(r.path); setQuery(''); setOpen(false); }}
                      className="w-full flex items-center gap-2.5 px-3 py-2 text-left hover:bg-[var(--color-tile-1)] text-callout text-[var(--color-ink)]"
                    >
                      {r.icon}
                      <span className="flex-1">{r.label}</span>
                      {r.section && <span className="text-fine text-[var(--color-text-muted-48)]">{r.section}</span>}
                    </button>
                  ))}
                </div>
              )}
              {results.actions.length > 0 && (
                <div className="py-1 border-t border-[var(--color-divider-soft)]">
                  <div className="px-3 py-1 text-fine text-[var(--color-text-muted-48)] uppercase tracking-wide">快捷操作</div>
                  {results.actions.map((a) => (
                    <button
                      key={a.label}
                      onClick={() => { if (a.path) { router.push(a.path); } setQuery(''); setOpen(false); }}
                      className="w-full flex items-center gap-2.5 px-3 py-2 text-left hover:bg-[var(--color-tile-1)] text-callout text-[var(--color-ink)]"
                    >
                      {a.icon}
                      <span>{a.label}</span>
                    </button>
                  ))}
                </div>
              )}
            </>
          )}
        </div>
      )}
    </div>
  );
}
