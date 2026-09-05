/** useAccountSwitcher 历史登录会话管理，登录自动保存账号，7 天过期。 */
'use client';

import { useCallback, useEffect, useSyncExternalStore } from 'react';
import { useAuth, type Menu } from './useAuth';
import { getUnreadCount } from '@/lib/api/message';
import { STORAGE_KEY, MAX_ACCOUNTS, EXPIRE_MS, type SavedAccount } from '@/lib/account-store';

const ACCOUNTS_EVENT = 'cognos-accounts-change';

function loadAccounts(): SavedAccount[] {
  if (typeof window === 'undefined') return [];
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

function saveAccounts(accounts: SavedAccount[]) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(accounts.slice(0, MAX_ACCOUNTS)));
  } catch { /* ignore */ }
}

/** 缓存快照引用，避免 useSyncExternalStore 因每次新数组引用触发无限重渲染。 */
let cachedRaw: string | null | undefined = undefined;
let cachedSnapshot: SavedAccount[] = [];

function subscribeAccounts(cb: () => void): () => void {
  window.addEventListener(ACCOUNTS_EVENT, cb);
  window.addEventListener('storage', cb);
  return () => {
    window.removeEventListener(ACCOUNTS_EVENT, cb);
    window.removeEventListener('storage', cb);
  };
}

function getAccountsSnapshot(): SavedAccount[] {
  if (typeof window === 'undefined') return [];
  const raw = localStorage.getItem(STORAGE_KEY);
  if (raw === cachedRaw) return cachedSnapshot;
  cachedRaw = raw;
  const all: SavedAccount[] = raw ? JSON.parse(raw) : [];
  const now = Date.now();
  cachedSnapshot = all
    .filter((a) => now - a.savedAt < EXPIRE_MS)
    .sort((a, b) => b.savedAt - a.savedAt);
  return cachedSnapshot;
}

function getAccountsServerSnapshot(): SavedAccount[] {
  return [];
}

/** 保存当前登录会话到历史列表（去重、7 天过期自动清除）。 */
export function useAccountSwitcher() {
  const { user, token, refreshToken, roles, permissions, menus, login, logout } = useAuth();
  const accounts = useSyncExternalStore(subscribeAccounts, getAccountsSnapshot, getAccountsServerSnapshot);

  // 清理过期账号（在 effect 中写 localStorage，不在 render 中产生副作用）
  useEffect(() => {
    const all = loadAccounts();
    if (all.length !== accounts.length) {
      saveAccounts(accounts);
      window.dispatchEvent(new Event(ACCOUNTS_EVENT));
    }
  }, [accounts]);

  const saveCurrent = useCallback(() => {
    if (!user || !token) return;
    const all = loadAccounts();
    const now = Date.now();
    // 剔除旧记录 + 过期记录
    const filtered = all.filter((a) => a.username !== user.username && now - a.savedAt < EXPIRE_MS);
    filtered.unshift({
      username: user.username,
      realName: user.real_name,
      token,
      refreshToken: refreshToken || '',
      roles,
      permissions,
      menus,
      savedAt: now,
    });
    saveAccounts(filtered);
    window.dispatchEvent(new Event(ACCOUNTS_EVENT));
  }, [user, token, refreshToken, roles, permissions, menus]);

  /** 删除单条记录并立即刷新列表。 */
  const removeAccount = useCallback((username: string) => {
    const all = loadAccounts().filter((a) => a.username !== username);
    saveAccounts(all);
    window.dispatchEvent(new Event(ACCOUNTS_EVENT));
  }, []);

  /** 直接切换到已保存的会话（免密登录）。切换后立即验证 token，冻结账号自动登出。 */
  const switchTo = useCallback(
    async (account: SavedAccount) => {
      const now = Date.now();
      if (now - account.savedAt > EXPIRE_MS) {
        logout();
        return false;
      }
      login(account.token, account.refreshToken, {
        id: 0,
        username: account.username,
        real_name: account.realName,
        phone: '',
        email: '',
      }, account.roles, account.permissions, account.menus as Menu[]);
      // 立即验证 token——账号可能已被冻结，后台返回 10001 触发自动登出
      try {
        await getUnreadCount(account.token);
      } catch (err: unknown) {
        const code = (err as { code?: number })?.code;
        if (code === 10001) {
          logout();
          removeAccount(account.username);
          return false;
        }
        /* 网络错误不处理，让后续 API 调用触发 */
      }
      return true;
    },
    [login, logout, removeAccount],
  );

  return { accounts, saveCurrent, switchTo, removeAccount, logout };
}
