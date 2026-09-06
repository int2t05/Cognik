/** PortalLayout — 统一 Shell。用户菜单 + 管理员可见的管理分区（来自 useAuth().menus）。 */

'use client';

import { useMemo } from 'react';
import { useTranslations } from 'next-intl';
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
  const t = useTranslations();
  const { permissions, menus } = useAuth();
  const { unreadCount } = useUnreadCount();
  const isAdmin = hasAdminAccess(permissions);

  // 后端菜单 → 管理分区 NavItem。菜单 path 映射到 i18n key，DB 的中文 name 不用。
  const menuLabel = (path: string): string => {
    const r = FRONTEND_ROUTES[path] || path;
    return t.has(`nav.menu.${r}`) ? t(`nav.menu.${r}`) : path;
  };
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
      label: menuLabel(m.path),
      path: FRONTEND_ROUTES[m.path] || m.path,
      icon: ICON_MAP[m.icon] || <Settings size={18} />,
      children: childMenus.filter((c: Menu) => c.parent_id === m.id).map((c: Menu) => ({
        label: menuLabel(c.path),
        path: FRONTEND_ROUTES[c.path] || c.path,
        icon: ICON_MAP[c.icon] || <Settings size={18} />,
      })),
    }));
  }, [isAdmin, menus, t]);

  const nav: NavSection[] = [
    {
      items: [
        { path: '/portal/chat', label: t('nav.chat'), icon: <Bot size={18} /> },
        { path: '/portal/tickets/new', label: t('nav.newTicket'), icon: <TicketPlus size={18} /> },
        { path: '/portal/tickets', label: t('nav.myTickets'), icon: <ListTodo size={18} /> },
        { path: '/portal/messages', label: t('nav.messages'), icon: <MessageSquare size={18} />, badge: unreadBadge(unreadCount) },
      ],
    },
    ...(isAdmin && adminItems.length > 0 ? [{ heading: t('nav.admin'), items: adminItems }] : []),
  ];

  return (
    <AppShell nav={nav}>
      {children}
    </AppShell>
  );
}
