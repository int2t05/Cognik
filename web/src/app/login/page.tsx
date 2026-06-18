/**
 * 登录页面 — Apple 风格居中卡片。
 *
 * 修复旧版错误提取用 err?.message 的问题：直接使用后端返回的 message。
 * 修复旧版路由判断基于 permissions.length 的 AMBIGUOUS 逻辑。
 * 参照 docs/TODO.md P0-2, P0-3。
 */

'use client';

import { useState, type FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import { AppleButton } from '@/components/ui/AppleButton';
import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/hooks/useToast';
import { apiFetch } from '@/lib/api/client';

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

      // 根据角色跳转（修复 P0-3）
      const adminRoles = ['系统管理员', 'admin', 'operator', 'knowledge_manager'];
      const isAdmin = data.roles.some((r) => adminRoles.includes(r));
      router.push(isAdmin ? '/admin/dashboard' : '/portal/chat');
    } catch (err: unknown) {
      // 直接提取后端 message（修复 P0-2）
      const message =
        err instanceof Error ? err.message : '登录失败，请重试';
      toast.error(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: '100vh',
        background: 'var(--bg-parchment)',
      }}
    >
      <div
        style={{
          width: 400,
          padding: 'var(--space-xxl)',
          background: 'var(--bg-canvas)',
          borderRadius: 'var(--radius-lg)',
          border: '1px solid var(--hairline)',
        }}
      >
        <h1
          style={{
            fontSize: 28,
            fontWeight: 600,
            letterSpacing: '-0.28px',
            textAlign: 'center',
            marginBottom: 8,
            color: 'var(--text-ink)',
          }}
        >
          OpsMind
        </h1>
        <p
          style={{
            fontSize: 15,
            color: 'var(--text-muted-48)',
            textAlign: 'center',
            marginBottom: 32,
          }}
        >
          运维数字员工系统
        </p>

        <form onSubmit={handleSubmit}>
          <div style={{ marginBottom: 20 }}>
            <label
              style={{
                display: 'block',
                fontSize: 14,
                fontWeight: 500,
                marginBottom: 6,
                color: 'var(--text-ink)',
              }}
            >
              用户名
            </label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              autoFocus
              style={{
                width: '100%',
                height: 44,
                padding: '0 16px',
                fontSize: 17,
                borderRadius: 'var(--radius-pill)',
                border: '1px solid var(--hairline)',
                background: 'var(--bg-canvas)',
                color: 'var(--text-ink)',
                outline: 'none',
                boxSizing: 'border-box',
              }}
            />
          </div>

          <div style={{ marginBottom: 24 }}>
            <label
              style={{
                display: 'block',
                fontSize: 14,
                fontWeight: 500,
                marginBottom: 6,
                color: 'var(--text-ink)',
              }}
            >
              密码
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              style={{
                width: '100%',
                height: 44,
                padding: '0 16px',
                fontSize: 17,
                borderRadius: 'var(--radius-pill)',
                border: '1px solid var(--hairline)',
                background: 'var(--bg-canvas)',
                color: 'var(--text-ink)',
                outline: 'none',
                boxSizing: 'border-box',
              }}
            />
          </div>

          <AppleButton type="submit" loading={loading} style={{ width: '100%' }}>
            登录
          </AppleButton>
        </form>
      </div>
    </div>
  );
}
