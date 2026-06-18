'use client';

import { useState, type FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import { changePassword } from '@/lib/api/auth';
import { AppleButton } from '@/components/ui/AppleButton';
import { AppleInput } from '@/components/ui/AppleInput';
import { useToast } from '@/hooks/useToast';
import styles from './page.module.css';

export default function ChangePasswordPage() {
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [loading, setLoading] = useState(false);
  const router = useRouter();
  const toast = useToast();

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!oldPassword || !newPassword) { toast.error('请填写所有字段'); return; }
    if (newPassword !== confirm) { toast.error('两次输入的新密码不一致'); return; }
    if (newPassword.length < 8) { toast.error('新密码至少 8 位，需含大小写字母和数字'); return; }
    setLoading(true);
    try {
      await changePassword(oldPassword, newPassword);
      toast.success('密码修改成功');
      setTimeout(() => router.push('/portal/chat'), 1000);
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : '修改失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className={styles.wrapper}>
      <div className={styles.card}>
        <h1 className={styles.title}>修改密码</h1>
        <form onSubmit={handleSubmit}>
          <AppleInput label="旧密码" type="password" value={oldPassword} onChange={(e) => setOldPassword(e.target.value)} autoComplete="current-password" />
          <AppleInput label="新密码" type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} autoComplete="new-password" />
          <AppleInput label="确认新密码" type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="new-password" />
          <div className={styles.submitArea}>
            <AppleButton type="submit" loading={loading} className={styles.fullWidth}>修改密码</AppleButton>
          </div>
        </form>
      </div>
    </div>
  );
}
