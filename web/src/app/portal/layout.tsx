'use client';

import { AuthProvider } from '@/hooks/useAuth';
import { PortalLayout as PortalLayoutUI } from '@/components/layout/PortalLayout';

export default function PortalLayout({ children }: { children: React.ReactNode }) {
  return (
    <AuthProvider>
      <PortalLayoutUI>{children}</PortalLayoutUI>
    </AuthProvider>
  );
}
