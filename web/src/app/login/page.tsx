/** 登录页面 — Apple 风格居中卡片。 */

'use client';

import { useState, type FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import Image from 'next/image';
import useSWR from 'swr';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput } from '@/components/ui/AppleInput';
import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/hooks/useToast';
import { getAppName } from '@/lib/config/defaults';
import { getPublicConfig } from '@/lib/api/config';
import { apiFetch } from '@/lib/api/client';
import { hasAdminAccess } from '@/lib/roles';
import { saveLoginAccount } from '@/lib/account-store';
import { LogIn } from 'lucide-react';

interface LoginResponse {
  access_token: string;
  refresh_token: string;
  user: { id: number; username: string; real_name: string; phone: string; email: string; first_login: boolean };
  roles: string[];
  permissions: string[];
  menus: never[];
}

export default function LoginPage() {
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
  const toast = useToast();

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!username.trim() || !password) {
      toast.error('请输入用户名和密码');
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
      // 直接提取后端 message
      const message =
        err instanceof Error ? err.message : '登录失败，请重试';
      toast.error(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex items-center justify-center min-h-screen bg-[var(--color-parchment)] p-4">
      <div className="w-full max-w-[420px] p-8 bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)] shadow-[var(--shadow-dialog)] card-entrance">
        <div className="text-center mb-8">
          <div className="mb-5">
            <Image src="/icon.svg" alt={displayName} width={56} height={56} className="mx-auto" priority />
          </div>
          <h1 className="text-hero font-semibold text-[var(--color-ink)] mb-2">
            {displayName}
          </h1>
          <p className="text-title text-[var(--color-text-muted-48)]">
            运维数字员工系统
          </p>
          <p className="text-caption text-[var(--color-text-muted-48)] mt-1">
            智能问答 · 申告管理 · 知识库
          </p>
        </div>

        <form onSubmit={handleSubmit}>
          <AppleInput
            label="用户名"
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter' && !loading) handleSubmit(e as unknown as FormEvent); }}
            autoComplete="username"
            autoFocus
          />
          <AppleInput
            label="密码"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter' && !loading) handleSubmit(e as unknown as FormEvent); }}
            autoComplete="current-password"
          />
          <div className="mt-8">
            <AppleButton type="submit" loading={loading} className="w-full" icon={<LogIn />}>
              登录
            </AppleButton>
          </div>
        </form>
      </div>
    </div>
  );
}
