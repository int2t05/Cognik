/** 登录页面 — Apple 风格居中卡片。 */

'use client';

import { useState, type FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import { Cpu } from 'lucide-react';
import { AppleButton } from '@/components/ui/AppleButton';
import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/hooks/useToast';
import { apiFetch } from '@/lib/api/client';
import { isAdminRole } from '@/lib/roles';

interface LoginResponse {
  access_token: string;
  refresh_token: string;
  user: { id: number; username: string; real_name: string; phone: string; email: string; first_login: boolean };
  roles: string[];
  permissions: string[];
  menus: never[];
}

export default function LoginPage() {
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

      // 根据角色跳转
      const isAdmin = isAdminRole(data.roles);
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
      <div className="w-full max-w-[420px] p-10 bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)] shadow-[var(--shadow-dialog)] card-entrance">
        <div className="text-center mb-10">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-[var(--radius-lg)] bg-[var(--color-accent)]/8 mb-6">
            <Cpu size={32} className="text-[var(--color-accent)]" />
          </div>
          <h1 className="text-hero font-semibold text-[var(--color-ink)] mb-2">
            OpsMind
          </h1>
          <p className="text-title text-[var(--color-text-muted-48)]">
            运维数字员工系统
          </p>
          <p className="text-caption text-[var(--color-text-muted-48)] mt-1">
            智能问答 · 申告管理 · 知识库
          </p>
        </div>

        <form onSubmit={handleSubmit}>
          <div className="mb-5">
            <label className="block text-caption font-medium text-[var(--color-ink)] mb-1.5">用户名</label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              autoFocus
              className="h-11 px-4 text-body rounded-lg border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none w-full transition focus:border-[var(--color-accent)] focus:shadow-[var(--focus-ring)]"
            />
          </div>

          <div className="mb-8">
            <label className="block text-caption font-medium text-[var(--color-ink)] mb-1.5">密码</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              className="h-11 px-4 text-body rounded-lg border border-[var(--color-hairline)] bg-[var(--color-canvas)] text-[var(--color-ink)] outline-none w-full transition focus:border-[var(--color-accent)] focus:shadow-[var(--focus-ring)]"
            />
          </div>

          <AppleButton type="submit" loading={loading} className="w-full">
            登录
          </AppleButton>
        </form>
      </div>
    </div>
  );
}
