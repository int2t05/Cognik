/** AuthContext — 全局认证状态管理。 */

'use client';
import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  type ReactNode,
} from 'react';
import { setTokenGetter } from '@/lib/api/client';
import { readAuth, writeAuth } from '@/lib/token-store';
import type { StoredAuth } from '@/lib/token-store';

interface User {
  id: number;
  username: string;
  real_name: string;
  phone: string;
  email: string;
}

export interface Menu {
  id: number;
  name: string;
  path: string;
  icon: string;
  parent_id: number;
  sort_order: number;
  children?: Menu[];
}

interface AuthState {
  token: string | null;
  refreshToken: string | null;
  user: User | null;
  roles: string[];
  permissions: string[];
  menus: Menu[];
  isLoggedIn: boolean;
}

interface AuthContextValue extends AuthState {
  login: (token: string, refreshToken: string, user: User, roles: string[], permissions: string[], menus: Menu[]) => void;
  logout: () => void;
  hasPermission: (perm: string) => boolean;
  setTokens: (accessToken: string, refreshToken: string) => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

// EMPTY_AUTH 空认证状态——SSR 与 hydration 首帧一致，避免 localStorage 读取导致水合不匹配。
const EMPTY_AUTH: AuthState = {
  token: null, refreshToken: null, user: null,
  roles: [], permissions: [], menus: [], isLoggedIn: false,
};

function loadAuthState(): AuthState {
  const stored = readAuth();
  if (stored) {
    return { ...stored, menus: (stored.menus || []) as Menu[], isLoggedIn: !!stored.token };
  }
  return { token: null, refreshToken: null, user: null, roles: [], permissions: [], menus: [], isLoggedIn: false };
}

function persistAuth(state: AuthState) {
  writeAuth(state as StoredAuth);
}

export function AuthProvider({ children }: { children: ReactNode }) {
  // 初始空状态：SSR 与 hydration 首帧一致，避免 localStorage 读取导致水合不匹配
  const [state, setState] = useState<AuthState>(EMPTY_AUTH);

  // hydration 后从 localStorage 加载认证状态，同步 token getter
  useEffect(() => {
    const stored = loadAuthState();
    setTokenGetter(() => stored.token);
    if (stored.isLoggedIn) {
      setState(stored);
    }
  }, []);

  // 同步 token/refreshToken 到 cookie（供 middleware 读取）
  useEffect(() => {
    if (state.token) {
      document.cookie = `access_token=${state.token}; path=/; SameSite=Lax; max-age=604800`;
      if (state.refreshToken) {
        document.cookie = `refresh_token=${state.refreshToken}; path=/; SameSite=Lax; max-age=604800`;
      }
    } else {
      document.cookie = 'access_token=; path=/; SameSite=Lax; max-age=0';
      document.cookie = 'refresh_token=; path=/; SameSite=Lax; max-age=0';
    }
  }, [state.token, state.refreshToken]);

  const login = useCallback(
    (token: string, refreshToken: string, user: User, roles: string[], permissions: string[], menus: Menu[]) => {
      const newState: AuthState = { token, refreshToken, user, roles, permissions, menus, isLoggedIn: true };
      // 同步写 cookie——router.push 触发中间件校验时必须能读到 token
      document.cookie = `access_token=${token}; path=/; SameSite=Lax; max-age=604800`;
      document.cookie = `refresh_token=${refreshToken}; path=/; SameSite=Lax; max-age=604800`;
      setTokenGetter(() => token);
      setState(newState);
      persistAuth(newState);
    },
    []
  );

  const logout = useCallback(() => {
    document.cookie = 'access_token=; path=/; SameSite=Lax; max-age=0';
    document.cookie = 'refresh_token=; path=/; SameSite=Lax; max-age=0';
    setTokenGetter(() => null);
    setState(EMPTY_AUTH);
    persistAuth(EMPTY_AUTH);
  }, []);

  const setTokens = useCallback(
    (accessToken: string, refreshToken: string) => {
      setTokenGetter(() => accessToken);
      setState((prev) => {
        const next = { ...prev, token: accessToken, refreshToken };
        persistAuth(next);
        return next;
      });
    },
    []
  );

  const hasPermission = useCallback(
    (perm: string) => state.permissions.includes(perm),
    [state.permissions]
  );

  return (
    <AuthContext.Provider value={{ ...state, login, logout, hasPermission, setTokens }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  // 在 SSR 阶段或 AuthProvider 未挂载时返回安全默认值
  if (!ctx) {
    if (typeof window === 'undefined') {
      return { token: null, refreshToken: null, user: null, roles: [], permissions: [], menus: [], isLoggedIn: false, login: () => {}, logout: () => {}, hasPermission: () => false, setTokens: () => {} };
    }
    throw new Error('useAuth 必须在 AuthProvider 内');
  }
  return ctx;
}
