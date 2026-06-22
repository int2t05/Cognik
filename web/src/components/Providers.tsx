'use client';

import { type ReactNode } from 'react';
import { AuthProvider } from '@/hooks/useAuth';
import { ThemeProvider } from '@/components/ThemeProvider';
import { ToastProvider } from '@/hooks/useToast';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { SWRConfig } from 'swr';

export function Providers({ children }: { children: ReactNode }) {
  return (
    <SWRConfig value={{ revalidateOnFocus: false, dedupingInterval: 5000 }}>
      <AuthProvider>
        <ThemeProvider>
          <ToastProvider>
            <ErrorBoundary>{children}</ErrorBoundary>
          </ToastProvider>
        </ThemeProvider>
      </AuthProvider>
    </SWRConfig>
  );
}
