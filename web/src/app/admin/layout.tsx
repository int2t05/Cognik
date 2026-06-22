'use client';

import { AdminLayout as AdminLayoutUI } from '@/components/layout/AdminLayout';

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  return <AdminLayoutUI>{children}</AdminLayoutUI>;
}
