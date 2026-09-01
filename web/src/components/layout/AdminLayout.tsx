/** AdminLayout — 后台管理布局。映射后端菜单 menus → NavItem，复用 AppShell（方向 C 统一 Shell）。 */

'use client';

import { useMemo } from 'react';
import { useAuth } from '@/hooks/useAuth';
import { useUnreadCount } from '@/hooks/useUnreadCount';
import { AppShell, type NavItem } from '@/components/layout/AppShell';
import { LayoutDashboard, Ticket, BookOpen, Users, Shield, Settings, ScrollText, MessageSquare, Cpu, FileText, User, Bot } from 'lucide-react';

// ICON_MAP 将后端菜单 icon 字段映射到 Lucide 图标
const ICON_MAP: Record<string, React.ReactNode> = {
  dashboard: <LayoutDashboard size={18} />,
  ticket: <Ticket size={18} />,
  knowledge: <BookOpen size={18} />,
  book: <BookOpen size={18} />,
  users: <Users size={18} />,
  user: <User size={18} />,
  role: <Shield size={18} />,
  shield: <Shield size={18} />,
  config: <Settings size={18} />,
  settings: <Settings size={18} />,
  audit: <ScrollText size={18} />,
  'file-text': <FileText size={18} />,
  message: <MessageSquare size={18} />,
  cpu: <Cpu size={18} />,
};

// FRONTEND_ROUTES 将后端菜单路径映射到实际前端路由
const FRONTEND_ROUTES: Record<string, string> = {
  '/admin/audit-logs': '/admin/audit',
  '/admin/model-config': '/admin/config/llm',
  '/admin/llm-config': '/admin/config/llm',
  '/admin/system-config': '/admin/config/system',
};

interface MenuItem { id: number; name: string; path: string; icon: string; parent_id: number; sort_order: number; type: string; }

export function AdminLayout({ children }: { children: React.ReactNode }) {
  const { menus } = useAuth();
  const { unreadCount } = useUnreadCount();

  const nav: NavItem[] = useMemo(() => {
    const top = menus.filter((m: MenuItem) => !m.parent_id);
    const childMenus = menus.filter((m: MenuItem) => m.parent_id);
    // 去重：多条菜单项可能映射到同一前端路由，按 sort_order 保留第一条
    const seen = new Set<string>();
    const deduped = top.filter((m: MenuItem) => {
      const r = FRONTEND_ROUTES[m.path] || m.path;
      if (seen.has(r)) return false;
      seen.add(r);
      return true;
    });
    return deduped.map((m: MenuItem) => ({
      label: m.name,
      path: FRONTEND_ROUTES[m.path] || m.path,
      icon: ICON_MAP[m.icon] || <Settings size={18} />,
      children: childMenus
        .filter((c: MenuItem) => c.parent_id === m.id)
        .map((c: MenuItem) => ({
          label: c.name,
          path: FRONTEND_ROUTES[c.path] || c.path,
          icon: ICON_MAP[c.icon] || <Settings size={18} />,
        })),
    }));
  }, [menus]);

  // 消息作为 nav 末项（跨域链接到门户消息 + 未读 badge）
  const navWithMessages: NavItem[] = [
    ...nav,
    {
      path: '/portal/messages',
      label: '消息',
      icon: <MessageSquare size={18} />,
      badge: unreadCount > 0 ? (
        <span className="bg-[var(--color-error)] text-[var(--color-canvas)] min-w-[16px] h-4 px-1 text-[10px] leading-none rounded-full flex items-center justify-center">
          {unreadCount > 99 ? '99+' : unreadCount}
        </span>
      ) : undefined,
    },
  ];

  return (
    <AppShell
      nav={navWithMessages}
      crossLink={{ label: '门户首页', path: '/portal/chat', icon: <Bot size={18} /> }}
    >
      {children}
    </AppShell>
  );
}
