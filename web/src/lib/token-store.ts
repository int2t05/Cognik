/**
 * token-store — 认证令牌的单一存储抽象。
 *
 * client.ts 和 useAuth 都需读写 'auth' localStorage 键。
 * 将序列化/反序列化逻辑集中于此，确保两者对数据形状的理解一致，
 * 避免一处变更另一处遗漏。
 */

const AUTH_KEY = 'auth';

export interface StoredAuth {
  token: string;
  refreshToken: string;
  user: {
    id: number;
    username: string;
    real_name: string;
    phone: string;
    email: string;
    first_login: boolean;
  };
  roles: string[];
  permissions: string[];
  menus: unknown[];
}

export function readAuth(): StoredAuth | null {
  if (typeof window === 'undefined') return null;
  try {
    const raw = localStorage.getItem(AUTH_KEY);
    if (!raw) return null;
    return JSON.parse(raw) as StoredAuth;
  } catch {
    return null;
  }
}

export function writeAuth(auth: StoredAuth): void {
  if (typeof window === 'undefined') return;
  try {
    localStorage.setItem(AUTH_KEY, JSON.stringify(auth));
  } catch { /* 存储满时静默失败 */ }
}

export function clearAuth(): void {
  if (typeof window === 'undefined') return;
  try {
    localStorage.removeItem(AUTH_KEY);
  } catch { /* 静默 */ }
}

/** 默认 token getter — 供 client.ts 在 AuthProvider 挂载前使用 */
export function defaultTokenGetter(): string | null {
  const auth = readAuth();
  return auth?.token ?? null;
}
