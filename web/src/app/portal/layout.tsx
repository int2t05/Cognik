'use client';

import { PortalLayout as PortalLayoutUI } from '@/components/layout/PortalLayout';

export default function PortalLayout({ children }: { children: React.ReactNode }) {
  return <PortalLayoutUI>{children}</PortalLayoutUI>;
}
