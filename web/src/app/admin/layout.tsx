'use client';

import { AuthProvider } from '@/hooks/useAuth';
import { AdminLayout as AdminLayoutUI } from '@/components/layout/AdminLayout';

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  return (
    <AuthProvider>
      <AdminLayoutUI>{children}</AdminLayoutUI>
    </AuthProvider>
  );
}
