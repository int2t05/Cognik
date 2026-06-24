/**
 * account-store — 历史登录账号本地存储（7 天过期）。
 *
 * 与 useAccountSwitcher hook 共享同一 localStorage key 和过期策略。
 */

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

/** 登录成功后调用：将当前账号写入历史列表顶部，去重 + 清除过期。 */
export function saveLoginAccount(account: Omit<SavedAccount, 'savedAt'>) {
  if (typeof window === 'undefined') return;
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    const all: SavedAccount[] = raw ? JSON.parse(raw) : [];
    const now = Date.now();

    // 剔除同名旧记录 + 过期记录
    const filtered = all.filter(
      (a) => a.username !== account.username && now - a.savedAt < EXPIRE_MS,
    );
    filtered.unshift({ ...account, savedAt: now });
    localStorage.setItem(STORAGE_KEY, JSON.stringify(filtered.slice(0, MAX_ACCOUNTS)));
  } catch { /* ignore */ }
}
