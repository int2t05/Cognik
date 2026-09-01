'use client';

import { PortalLayout as PortalLayoutUI } from '@/components/layout/PortalLayout';

// Admin 路由复用统一 PortalLayout（管理员分区由权限自动渲染）。
// 无 ChatStreamProvider——admin 不需要 chat 流式状态。
export default function AdminLayout({ children }: { children: React.ReactNode }) {
  return <PortalLayoutUI>{children}</PortalLayoutUI>;
}
