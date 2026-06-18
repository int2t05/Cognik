/** 登录页面 — Apple 风格居中卡片。 */

'use client';

import { useState, type FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import { AppleButton } from '@/components/ui/AppleButton';
import { useAuth } from '@/hooks/useAuth';
import { useToast } from '@/hooks/useToast';
import { apiFetch } from '@/lib/api/client';
import { isAdminRole } from '@/lib/roles';
import styles from './page.module.css';

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
    <div className={styles.wrapper}>
      <div className={styles.card}>
        <h1 className={styles.title}>OpsMind</h1>
        <p className={styles.subtitle}>运维数字员工系统</p>

        <form onSubmit={handleSubmit}>
          <div className={styles.field}>
            <label className={styles.label}>用户名</label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoComplete="username"
              autoFocus
              className={styles.input}
            />
          </div>

          <div className={styles.fieldLast}>
            <label className={styles.label}>密码</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              className={styles.input}
            />
          </div>

          <AppleButton type="submit" loading={loading} className={styles.fullWidth}>
            登录
          </AppleButton>
        </form>
      </div>
    </div>
  );
}
