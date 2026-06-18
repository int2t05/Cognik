'use client';

import { type ReactNode } from 'react';
import { ThemeProvider } from '@/components/ThemeProvider';
import { ToastProvider } from '@/hooks/useToast';
import { ErrorBoundary } from '@/components/ErrorBoundary';

export function Providers({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider>
      <ToastProvider>
        <ErrorBoundary>{children}</ErrorBoundary>
      </ToastProvider>
    </ThemeProvider>
  );
}
