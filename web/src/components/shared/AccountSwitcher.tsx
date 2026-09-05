/**
 * AccountSwitcher — 切换账号弹出框。基于 shadcn DropdownMenu（Radix 处理 overlay/escape/portal/focus），
 * 列出历史登录会话，有效会话点击切换，过期/新增跳登录页。内含修改密码入口。
 */
'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';
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
import { translateError } from '@/lib/api/error';
import { toast } from 'sonner';

interface Props {
  /** 触发按钮的 className（由调用方控制样式）。 */
  className?: string;
  /** 是否仅显示图标（折叠态）。 */
  iconOnly?: boolean;
}

export function AccountSwitcher({ className, iconOnly }: Props) {
  const t = useTranslations();
  const { accounts, switchTo, removeAccount, logout } = useAccountSwitcher();
  const router = useRouter();
  const [pwdOpen, setPwdOpen] = useState(false);
  const [oldPwd, setOldPwd] = useState('');
  const [newPwd, setNewPwd] = useState('');
  const [confirmPwd, setConfirmPwd] = useState('');
  const [pwdLoading, setPwdLoading] = useState(false);

  const resetPwdForm = () => { setOldPwd(''); setNewPwd(''); setConfirmPwd(''); };

  const handleChangePassword = async () => {
    if (!oldPwd || !newPwd || !confirmPwd) { toast.error(t('account.fillAll')); return; }
    if (newPwd === oldPwd) { toast.error(t('account.sameAsOld')); return; }
    if (newPwd !== confirmPwd) { toast.error(t('account.mismatch')); return; }
    if (newPwd.length < 8) { toast.error(t('account.tooShort')); return; }
    setPwdLoading(true);
    try {
      await changePassword(oldPwd, newPwd);
      toast.success(t('account.changeSuccess'));
      setPwdOpen(false);
      resetPwdForm();
    } catch (err: unknown) {
      toast.error(translateError(err, t, t('account.changeFailed')));
    } finally {
      setPwdLoading(false);
    }
  };

  const handleSwitch = async (account: (typeof accounts)[0]) => {
    const ok = await switchTo(account);
    if (ok) {
      router.push('/portal/chat');
    } else {
      toast.warning(t('account.accountFrozen', { name: account.realName || account.username }));
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
        <IconButton variant="menu" aria-label={t('account.switchTitle')} className={className}>
          <UserPlus size={18} />
          {!iconOnly && t('account.switchTitle')}
        </IconButton>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64 p-0">
        <div className="px-4 py-3 border-b border-[var(--color-divider-soft)]">
          <p className="text-fine text-[var(--color-text-muted-48)]">{t('account.switchTitle')}</p>
        </div>

        <div className="max-h-[280px] overflow-y-auto overscroll-behavior-contain">
          {accounts.length === 0 ? (
            <p className="px-4 py-6 text-caption text-[var(--color-text-muted-48)] text-center">
              {t('account.noAccounts')}
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
                    label={t('account.removeAccount', { name: a.username })}
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
            {t('account.changePassword')}
          </IconButton>
          <IconButton variant="menu" onClick={handleNewLogin} className="w-full justify-start font-semibold">
            <LogIn size={18} />
            {t('account.otherLogin')}
          </IconButton>
        </div>
      </DropdownMenuContent>

      <Dialog open={pwdOpen} onOpenChange={(open) => { setPwdOpen(open); if (!open) resetPwdForm(); }}>
        <DialogContent showCloseButton={false} className="max-w-sm">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-[15px]">
              <KeyRound size={18} />
              {t('account.changePassword')}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <Field label={t('account.oldPassword')} required><Input type="password" value={oldPwd} onChange={(e) => setOldPwd(e.target.value)} autoComplete="current-password" disabled={pwdLoading} autoFocus /></Field>
            <Field label={t('account.newPassword')} required><Input type="password" value={newPwd} onChange={(e) => setNewPwd(e.target.value)} autoComplete="new-password" disabled={pwdLoading} placeholder={t('account.passwordHint')} /></Field>
            <Field label={t('account.confirmPassword')} required><Input type="password" value={confirmPwd} onChange={(e) => setConfirmPwd(e.target.value)} autoComplete="new-password" disabled={pwdLoading} /></Field>
          </div>
          <DialogFooter className="flex-row justify-end gap-2">
            <IconButton variant="ghost" size="sm" onClick={() => setPwdOpen(false)} disabled={pwdLoading}>{t('common.cancel')}</IconButton>
            <IconButton size="sm" onClick={handleChangePassword} disabled={pwdLoading}>{pwdLoading ? <Loader2 className="animate-spin" size={14} /> : <KeyRound size={14} />}{t('account.confirmChange')}</IconButton>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </DropdownMenu>
  );
}
