'use client';

import { PortalLayout as PortalLayoutUI } from '@/components/layout/PortalLayout';
import { ChatStreamProvider } from '@/contexts/ChatStreamProvider';

export default function PortalLayout({ children }: { children: React.ReactNode }) {
  return (
    <ChatStreamProvider>
      <PortalLayoutUI>{children}</PortalLayoutUI>
    </ChatStreamProvider>
  );
}
