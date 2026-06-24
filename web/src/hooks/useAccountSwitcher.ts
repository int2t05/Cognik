/**
 * useAccountSwitcher — 历史登录会话管理。
 *
 * 登录时自动保存账号（token + 角色权限），7 天过期。
 * 过期账号点击后需重新输入密码。
 */
'use client';

import { useCallback, useMemo } from 'react';
import { useAuth } from './useAuth';

const STORAGE_KEY = 'opsmind-accounts';
const MAX_ACCOUNTS = 5;
const EXPIRE_MS = 7 * 24 * 3600 * 1000; // 7 天

export interface SavedAccount {
  username: string;
  realName: string;
  token: string;
  refreshToken: string;
  roles: string[];
  permissions: string[];
  menus: unknown[];
  savedAt: number;
}

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

/** 保存当前登录会话到历史列表（去重、7 天过期自动清除）。 */
export function useAccountSwitcher() {
  const { user, token, refreshToken, roles, permissions, menus, login, logout } = useAuth();

  const accounts = useMemo(() => {
    const all = loadAccounts();
    const now = Date.now();
    // 过期清除 + 按时间倒序
    const valid = all.filter((a) => now - a.savedAt < EXPIRE_MS).sort((a, b) => b.savedAt - a.savedAt);
    if (valid.length !== all.length) saveAccounts(valid);
    return valid;
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

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
  }, [user, token, refreshToken, roles, permissions, menus]);

  /** 直接切换到已保存的会话（免密登录）。 */
  const switchTo = useCallback(
    (account: SavedAccount) => {
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
        first_login: false,
      }, account.roles, account.permissions, account.menus as never[]);
      return true;
    },
    [login, logout],
  );

  /** 删除单条记录。 */
  const removeAccount = useCallback((username: string) => {
    const all = loadAccounts().filter((a) => a.username !== username);
    saveAccounts(all);
  }, []);

  return { accounts, saveCurrent, switchTo, removeAccount, logout };
}
