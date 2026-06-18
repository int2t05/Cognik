'use client';

import { AuthProvider } from '@/hooks/useAuth';

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  return <AuthProvider>{children}</AuthProvider>;
}
