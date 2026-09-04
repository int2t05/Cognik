'use client';

import { type ReactNode } from 'react';
import { Tooltip } from 'radix-ui';
import { AuthProvider } from '@/hooks/useAuth';
import { ThemeProvider } from '@/components/ThemeProvider';
import { Toaster } from '@/components/ui/sonner';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { SWRConfig } from 'swr';

export function Providers({ children }: { children: ReactNode }) {
  return (
    <SWRConfig value={{ revalidateOnFocus: false, dedupingInterval: 5000 }}>
      <AuthProvider>
        <ThemeProvider>
          <Tooltip.Provider delayDuration={300}>
            <ErrorBoundary>{children}</ErrorBoundary>
          </Tooltip.Provider>
          <Toaster />
        </ThemeProvider>
      </AuthProvider>
    </SWRConfig>
  );
}
