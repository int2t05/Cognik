'use client';

import { useState, type FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import { changePassword } from '@/lib/api/auth';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Field } from '@/components/ui/form-field';
import { toast } from 'sonner';
import { errorMessage } from '@/lib/api/error';
import { Key, Loader2 } from 'lucide-react';

export default function ChangePasswordPage() {
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [loading, setLoading] = useState(false);
  const router = useRouter();

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
      toast.error(errorMessage(err, '修改失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex items-center justify-center min-h-screen bg-[var(--color-parchment)]">
      <div className="w-full max-w-form p-6 bg-[var(--color-canvas)] rounded-[var(--radius-lg)] border border-[var(--color-hairline)]">
        <h1 className="text-display-md font-semibold text-[var(--color-ink)] text-center mb-5">修改密码</h1>
        <form onSubmit={handleSubmit}>
          <Field label="旧密码"><Input type="password" value={oldPassword} onChange={(e) => setOldPassword(e.target.value)} autoComplete="current-password" disabled={loading} /></Field>
          <Field label="新密码"><Input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} autoComplete="new-password" disabled={loading} /></Field>
          <Field label="确认新密码"><Input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="new-password" disabled={loading} /></Field>
          <div className="mt-6">
            <Button size="lg" type="submit" disabled={loading} className="w-full">{loading ? <Loader2 className="animate-spin" size={18} /> : <Key size={18} />}修改密码</Button>
          </div>
        </form>
      </div>
    </div>
  );
}
