/** 登录页面 — 居中卡片。 */

'use client';

import { useState, type FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';
import Image from 'next/image';
import useSWR from 'swr';
import { IconButton } from '@/components/ui/icon-button';
import { Input } from '@/components/ui/input';
import { Field } from '@/components/ui/form-field';
import { LocaleSwitcher } from '@/components/shared/LocaleSwitcher';
import { useAuth, type Menu } from '@/hooks/useAuth';
import { useTheme } from '@/hooks/useTheme';
import { toast } from 'sonner';
import { translateError } from '@/lib/api/error';
import { getAppName } from '@/lib/config/defaults';
import { getPublicConfig } from '@/lib/api/config';
import { apiFetch } from '@/lib/api/client';
import { hasAdminAccess } from '@/lib/roles';
import { saveLoginAccount } from '@/lib/account-store';
import { LogIn, Loader2 } from 'lucide-react';

interface LoginResponse {
  access_token: string;
  refresh_token: string;
  user: { id: number; username: string; real_name: string; phone: string; email: string };
  roles: string[];
  permissions: string[];
  menus: Menu[];
}

export default function LoginPage() {
  const t = useTranslations();
  const { data: appName } = useSWR('public-app-name', () => getPublicConfig('app_name'), {
    revalidateOnFocus: true,
    refreshInterval: 900_000, // 15 分钟轮询
    dedupingInterval: 0, // 每次页面聚焦都重新获取，确保刷新即可更新
  });
  const displayName = (typeof appName === 'string' ? appName : undefined) || getAppName();

  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const router = useRouter();
  const { login } = useAuth();
  const { theme } = useTheme();

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!username.trim() || !password) {
      toast.error(t('auth.fillUsernamePassword'));
      return;
    }

    setLoading(true);
    try {
      const data = await apiFetch<LoginResponse>('/api/v1/auth/login', {
        method: 'POST',
        body: JSON.stringify({ username: username.trim(), password }),
      });

      login(data.access_token, data.refresh_token, data.user, data.roles, data.permissions, data.menus);

      // 保存登录会话到历史列表（7 天有效）
      saveLoginAccount({
        username: data.user.username,
        realName: data.user.real_name,
        token: data.access_token,
        refreshToken: data.refresh_token,
        roles: data.roles,
        permissions: data.permissions,
        menus: data.menus,
      });

      // 根据角色跳转
      const isAdmin = hasAdminAccess(data.permissions);
      router.push(isAdmin ? '/admin/dashboard' : '/portal/chat');
    } catch (err: unknown) {
      toast.error(translateError(err, t, t('auth.loginFailed')));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="relative flex items-center justify-center min-h-screen bg-[var(--color-parchment)] p-4">
      <div className="absolute top-4 right-4"><LocaleSwitcher /></div>
      <div className="w-full max-w-[420px] p-6 bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)] shadow-[var(--shadow-dialog)] card-entrance">
        <div className="text-center mb-6">
          <div className="mb-4">
            <Image src={theme === 'dark' ? '/icon-dark.svg' : '/icon-light.svg'} alt={displayName} width={48} height={48} className="mx-auto" priority />
          </div>
          <h1 className="text-display-md font-semibold text-[var(--color-ink)] mb-1.5">
            {displayName}
          </h1>
          <p className="text-callout text-[var(--color-text-muted-48)]">
            {t('auth.systemTitle')}
          </p>
          <p className="text-caption text-[var(--color-text-muted-48)] mt-1">
            {t('auth.systemFeatures')}
          </p>
        </div>

        <form onSubmit={handleSubmit}>
          <Field label={t('auth.username')}>
            <Input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter' && !loading) handleSubmit(e as unknown as FormEvent); }}
              autoComplete="username"
              autoFocus
            />
          </Field>
          <Field label={t('auth.password')}>
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter' && !loading) handleSubmit(e as unknown as FormEvent); }}
              autoComplete="current-password"
            />
          </Field>
          <div className="mt-8">
            <IconButton size="lg" type="submit" disabled={loading} className="w-full">{loading ? <Loader2 className="animate-spin" size={18} /> : <LogIn size={18} />}{t('auth.login')}</IconButton>
          </div>
        </form>
      </div>
    </div>
  );
}
