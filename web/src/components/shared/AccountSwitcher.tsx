/**
 * AccountSwitcher — 切换账号弹出框。基于 shadcn DropdownMenu（Radix 处理 overlay/escape/portal/focus），
 * 列出历史登录会话，有效会话点击切换，过期/新增跳登录页。内含修改密码入口。
 */
'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { UserPlus, Trash2, LogIn, KeyRound, Loader2 } from 'lucide-react';
import { IconButton } from '@/components/ui/icon-button';
import { Input } from '@/components/ui/input';
import { Field } from '@/components/ui/form-field';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useAccountSwitcher } from '@/hooks/useAccountSwitcher';
import { changePassword } from '@/lib/api/auth';
import { errorMessage } from '@/lib/api/error';
import { toast } from 'sonner';

interface Props {
  /** 触发按钮的 className（由调用方控制样式）。 */
  className?: string;
  /** 是否仅显示图标（折叠态）。 */
  iconOnly?: boolean;
}

export function AccountSwitcher({ className, iconOnly }: Props) {
  const { accounts, switchTo, removeAccount, logout } = useAccountSwitcher();
  const router = useRouter();
  const [pwdOpen, setPwdOpen] = useState(false);
  const [oldPwd, setOldPwd] = useState('');
  const [newPwd, setNewPwd] = useState('');
  const [confirmPwd, setConfirmPwd] = useState('');
  const [pwdLoading, setPwdLoading] = useState(false);

  const resetPwdForm = () => { setOldPwd(''); setNewPwd(''); setConfirmPwd(''); };

  const handleChangePassword = async () => {
    if (!oldPwd || !newPwd || !confirmPwd) { toast.error('请填写所有字段'); return; }
    if (newPwd === oldPwd) { toast.error('新密码不能与旧密码相同'); return; }
    if (newPwd !== confirmPwd) { toast.error('两次输入的新密码不一致'); return; }
    if (newPwd.length < 8) { toast.error('新密码至少 8 位，需含大小写字母和数字'); return; }
    setPwdLoading(true);
    try {
      await changePassword(oldPwd, newPwd);
      toast.success('密码修改成功');
      setPwdOpen(false);
      resetPwdForm();
    } catch (err: unknown) {
      toast.error(errorMessage(err, '修改失败'));
    } finally {
      setPwdLoading(false);
    }
  };

  const handleSwitch = async (account: (typeof accounts)[0]) => {
    const ok = await switchTo(account);
    if (ok) {
      router.push('/portal/chat');
    } else {
      toast.warning(`账号「${account.realName || account.username}」已被冻结或失效，已自动移除`);
      logout();
      router.push('/login');
    }
  };

  const handleNewLogin = () => {
    logout();
    router.push('/login');
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <IconButton variant="menu" aria-label="切换账号" className={className}>
          <UserPlus size={18} />
          {!iconOnly && '切换账号'}
        </IconButton>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64 p-0">
        <div className="px-4 py-3 border-b border-[var(--color-divider-soft)]">
          <p className="text-fine text-[var(--color-text-muted-48)]">切换账号</p>
        </div>

        <div className="max-h-[280px] overflow-y-auto overscroll-behavior-contain">
          {accounts.length === 0 ? (
            <p className="px-4 py-6 text-caption text-[var(--color-text-muted-48)] text-center">
              暂无历史账号
            </p>
          ) : (
            accounts.map((a) => {
              return (
                <DropdownMenuItem
                  key={a.username}
                  onSelect={() => handleSwitch(a)}
                  className="w-full flex items-center gap-3 px-4 py-3 text-caption"
                >
                  <span className="w-8 h-8 rounded-full bg-[var(--color-accent)]/10 flex items-center justify-center text-caption font-semibold text-[var(--color-accent)] shrink-0">
                    {a.realName?.[0] || a.username?.[0] || '?'}
                  </span>
                  <span className="flex-1 min-w-0">
                    <span className="block truncate text-[var(--color-ink)]">{a.realName || a.username}</span>
                    <span className="block text-fine text-[var(--color-text-muted-48)]">
                      {a.username}
                    </span>
                  </span>
                  <IconButton
                    label={`移除 ${a.username}`}
                    danger
                    size="icon-sm"
                    onClick={(e: React.MouseEvent) => { e.stopPropagation(); removeAccount(a.username); }}
                    onPointerDown={(e: React.PointerEvent) => e.stopPropagation()}
                  >
                    <Trash2 size={14} />
                  </IconButton>
                </DropdownMenuItem>
              );
            })
          )}
        </div>

        <div className="border-t border-[var(--color-divider-soft)]">
          <IconButton variant="menu" onClick={() => setPwdOpen(true)} className="w-full justify-start">
            <KeyRound size={18} />
            修改密码
          </IconButton>
          <IconButton variant="menu" onClick={handleNewLogin} className="w-full justify-start font-semibold">
            <LogIn size={18} />
            其他账号登录
          </IconButton>
        </div>
      </DropdownMenuContent>

      <Dialog open={pwdOpen} onOpenChange={(open) => { setPwdOpen(open); if (!open) resetPwdForm(); }}>
        <DialogContent showCloseButton={false} className="max-w-sm">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-[15px]">
              <KeyRound size={18} />
              修改密码
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <Field label="旧密码" required><Input type="password" value={oldPwd} onChange={(e) => setOldPwd(e.target.value)} autoComplete="current-password" disabled={pwdLoading} autoFocus /></Field>
            <Field label="新密码" required><Input type="password" value={newPwd} onChange={(e) => setNewPwd(e.target.value)} autoComplete="new-password" disabled={pwdLoading} placeholder="至少 8 位，含大小写字母和数字" /></Field>
            <Field label="确认新密码" required><Input type="password" value={confirmPwd} onChange={(e) => setConfirmPwd(e.target.value)} autoComplete="new-password" disabled={pwdLoading} /></Field>
          </div>
          <DialogFooter className="flex-row justify-end gap-2">
            <IconButton variant="ghost" size="sm" onClick={() => setPwdOpen(false)} disabled={pwdLoading}>取消</IconButton>
            <IconButton size="sm" onClick={handleChangePassword} disabled={pwdLoading}>{pwdLoading ? <Loader2 className="animate-spin" size={14} /> : <KeyRound size={14} />}确认修改</IconButton>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </DropdownMenu>
  );
}
