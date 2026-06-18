'use client';

import { AuthProvider } from '@/hooks/useAuth';

export default function PortalLayout({ children }: { children: React.ReactNode }) {
  return <AuthProvider>{children}</AuthProvider>;
}
