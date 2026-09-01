/** PortalLayout — 门户端布局。映射静态 NAV 到 AppShell（方向 C 统一 Shell）。 */

'use client';

import { useAuth } from '@/hooks/useAuth';
import { useUnreadCount } from '@/hooks/useUnreadCount';
import { hasAdminAccess } from '@/lib/roles';
import { AppShell, type NavItem } from '@/components/layout/AppShell';
import { Bot, TicketPlus, ListTodo, MessageSquare, Shield } from 'lucide-react';

export function PortalLayout({ children }: { children: React.ReactNode }) {
  const { permissions } = useAuth();
  const { unreadCount } = useUnreadCount();
  const isAdmin = hasAdminAccess(permissions);

  const nav: NavItem[] = [
    { path: '/portal/chat', label: '智能问答', icon: <Bot size={18} /> },
    { path: '/portal/tickets/new', label: '提交申告', icon: <TicketPlus size={18} /> },
    { path: '/portal/tickets', label: '我的申告', icon: <ListTodo size={18} /> },
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
      nav={nav}
      crossLink={isAdmin ? { label: '后台管理', path: '/admin/dashboard', icon: <Shield size={18} /> } : undefined}
    >
      {children}
    </AppShell>
  );
}
