/**
 * AccountSwitcher — 切换账号弹出框。基于 shadcn DropdownMenu（Radix 处理 overlay/escape/portal/focus），
 * 替代原手写 useState + outside-click。列出历史登录会话，有效会话点击切换，过期/新增跳登录页。
 */
'use client';

import { useRouter } from 'next/navigation';
import { UserPlus, Trash2, LogIn } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useAccountSwitcher } from '@/hooks/useAccountSwitcher';
import { useToast } from '@/hooks/useToast';

interface Props {
  /** 触发按钮的 className（由调用方控制样式）。 */
  className?: string;
  /** 是否仅显示图标（折叠态）。 */
  iconOnly?: boolean;
}

export function AccountSwitcher({ className, iconOnly }: Props) {
  const { accounts, switchTo, removeAccount, logout } = useAccountSwitcher();
  const router = useRouter();
  const toast = useToast();

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
        <Button variant="menu" aria-label="切换账号" className={className}>
          <UserPlus size={18} />
          {!iconOnly && '切换账号'}
        </Button>
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
              const expired = Date.now() - a.savedAt > 7 * 24 * 3600 * 1000;
              return (
                <button
                  key={a.username}
                  onClick={() => handleSwitch(a)}
                  className={`w-full flex items-center gap-3 px-4 py-3 text-left border-0 bg-transparent cursor-pointer transition hover:bg-[var(--color-divider-soft)] text-caption focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent-focus)] ${expired ? 'opacity-50' : ''}`}
                >
                  <span className="w-8 h-8 rounded-full bg-[var(--color-accent)]/10 flex items-center justify-center text-caption font-semibold text-[var(--color-accent)] shrink-0">
                    {a.realName?.[0] || a.username?.[0] || '?'}
                  </span>
                  <span className="flex-1 min-w-0">
                    <span className="block truncate text-[var(--color-ink)]">{a.realName || a.username}</span>
                    <span className="block text-fine text-[var(--color-text-muted-48)]">
                      {a.username}{expired ? ' · 已过期' : ''}
                    </span>
                  </span>
                  <span
                    role="button"
                    tabIndex={0}
                    onClick={(e) => { e.stopPropagation(); removeAccount(a.username); }}
                    onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); e.stopPropagation(); removeAccount(a.username); } }}
                    aria-label={`移除 ${a.username}`}
                    className="p-1 text-[var(--color-text-muted-48)] hover:text-[var(--color-error)] transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent-focus)] rounded-full"
                  >
                    <Trash2 size={14} />
                  </span>
                </button>
              );
            })
          )}
        </div>

        <div className="border-t border-[var(--color-divider-soft)]">
          <Button variant="menu" onClick={handleNewLogin} className="w-full justify-start font-semibold">
            <LogIn size={18} />
            其他账号登录
          </Button>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
