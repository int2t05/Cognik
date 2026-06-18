'use client';

import { AuthProvider } from '@/hooks/useAuth';

export default function ChangePasswordLayout({ children }: { children: React.ReactNode }) {
  return <AuthProvider>{children}</AuthProvider>;
}
