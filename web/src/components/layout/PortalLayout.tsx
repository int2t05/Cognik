/** PortalLayout — 统一 Shell。用户菜单 + 管理员可见的管理分区（来自 useAuth().menus）。 */

'use client';

import { useMemo } from 'react';
import { useAuth, type Menu } from '@/hooks/useAuth';
import { useUnreadCount } from '@/hooks/useUnreadCount';
import { hasAdminAccess } from '@/lib/roles';
import { AppShell, type NavSection, type NavItem } from '@/components/layout/AppShell';
import { Bot, TicketPlus, ListTodo, MessageSquare, Shield } from 'lucide-react';

// 后端菜单 icon 字段 → Lucide 图标（管理分区用）
import { LayoutDashboard, Ticket, BookOpen, Users, Settings, ScrollText, Cpu, FileText, User } from 'lucide-react';
const ICON_MAP: Record<string, React.ReactNode> = {
  dashboard: <LayoutDashboard size={18} />, ticket: <Ticket size={18} />,
  knowledge: <BookOpen size={18} />, book: <BookOpen size={18} />,
  users: <Users size={18} />, user: <User size={18} />,
  role: <Shield size={18} />, shield: <Shield size={18} />,
  config: <Settings size={18} />, settings: <Settings size={18} />,
  audit: <ScrollText size={18} />, 'file-text': <FileText size={18} />,
  message: <MessageSquare size={18} />, cpu: <Cpu size={18} />,
};

const FRONTEND_ROUTES: Record<string, string> = {
  '/admin/audit-logs': '/admin/audit',
  '/admin/model-config': '/admin/config/llm',
  '/admin/llm-config': '/admin/config/llm',
  '/admin/system-config': '/admin/config/system',
};

function unreadBadge(count: number) {
  return count > 0 ? (
    <span className="bg-[var(--color-error)] text-[var(--color-canvas)] min-w-[16px] h-4 px-1 text-fine leading-none rounded-full flex items-center justify-center">
      {count > 99 ? '99+' : count}
    </span>
  ) : undefined;
}

export function PortalLayout({ children }: { children: React.ReactNode }) {
  const { permissions, menus } = useAuth();
  const { unreadCount } = useUnreadCount();
  const isAdmin = hasAdminAccess(permissions);

  // 后端菜单 → 管理分区 NavItem（去重 + 路由别名 + 子菜单）
  const adminItems: NavItem[] = useMemo(() => {
    if (!isAdmin || !menus.length) return [];
    const top = menus.filter((m: Menu) => !m.parent_id);
    const childMenus = menus.filter((m: Menu) => m.parent_id);
    const seen = new Set<string>();
    return top.filter((m: Menu) => {
      const r = FRONTEND_ROUTES[m.path] || m.path;
      if (seen.has(r)) return false;
      seen.add(r);
      return true;
    }).map((m: Menu) => ({
      label: m.name,
      path: FRONTEND_ROUTES[m.path] || m.path,
      icon: ICON_MAP[m.icon] || <Settings size={18} />,
      children: childMenus.filter((c: Menu) => c.parent_id === m.id).map((c: Menu) => ({
        label: c.name,
        path: FRONTEND_ROUTES[c.path] || c.path,
        icon: ICON_MAP[c.icon] || <Settings size={18} />,
      })),
    }));
  }, [isAdmin, menus]);

  const nav: NavSection[] = [
    {
      items: [
        { path: '/portal/chat', label: '智能问答', icon: <Bot size={18} /> },
        { path: '/portal/tickets/new', label: '提交工单', icon: <TicketPlus size={18} /> },
        { path: '/portal/tickets', label: '我的工单', icon: <ListTodo size={18} /> },
        { path: '/portal/messages', label: '消息', icon: <MessageSquare size={18} />, badge: unreadBadge(unreadCount) },
      ],
    },
    ...(isAdmin && adminItems.length > 0 ? [{ heading: '管理', items: adminItems }] : []),
  ];

  return (
    <AppShell nav={nav}>
      {children}
    </AppShell>
  );
}
