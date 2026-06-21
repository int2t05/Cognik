/** AuthContext — 全局认证状态管理。 */

'use client';
import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  useLayoutEffect,
  type ReactNode,
} from 'react';
import { setTokenGetter } from '@/lib/api/client';

interface User {
  id: number;
  username: string;
  real_name: string;
  phone: string;
  email: string;
  first_login: boolean;
}

interface Menu {
  id: number;
  name: string;
  path: string;
  icon: string;
  parent_id: number;
  sort_order: number;
  type: string;
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

function loadAuthState(): AuthState {
  if (typeof window === 'undefined') {
    return { token: null, refreshToken: null, user: null, roles: [], permissions: [], menus: [], isLoggedIn: false };
  }
  try {
    const stored = localStorage.getItem('auth');
    if (stored) return JSON.parse(stored);
  } catch { /* ignore */ }
  return { token: null, refreshToken: null, user: null, roles: [], permissions: [], menus: [], isLoggedIn: false };
}

function persistAuth(state: AuthState) {
  try {
    localStorage.setItem('auth', JSON.stringify(state));
  } catch { /* ignore */ }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>(loadAuthState);

  // 同步 token/refreshToken 到 cookie（供 middleware 读取）
  useEffect(() => {
    if (state.token) {
      document.cookie = `access_token=${state.token}; path=/; SameSite=Lax; max-age=604800`;
      if (state.refreshToken) {
        document.cookie = `refresh_token=${state.refreshToken}; path=/; SameSite=Lax; max-age=604800`;
      }
    } else {
      // token 变 null（登出）时清除 cookie
      document.cookie = 'access_token=; path=/; SameSite=Lax; max-age=0';
      document.cookie = 'refresh_token=; path=/; SameSite=Lax; max-age=0';
    }
  }, [state.token, state.refreshToken]);

  // 同步 token 到 apiFetch（自动附加 Authorization header）。
  // 用 layout effect 保证子页面的 SWR 首次请求前已经拿到 token。
  useLayoutEffect(() => {
    setTokenGetter(() => state.token);
  }, [state.token]);

  const login = useCallback(
    (token: string, refreshToken: string, user: User, roles: string[], permissions: string[], menus: Menu[]) => {
      const newState: AuthState = { token, refreshToken, user, roles, permissions, menus, isLoggedIn: true };
      setState(newState);
      persistAuth(newState);
    },
    []
  );

  const logout = useCallback(() => {
    const empty: AuthState = { token: null, refreshToken: null, user: null, roles: [], permissions: [], menus: [], isLoggedIn: false };
    setState(empty);
    persistAuth(empty);
  }, []);

  const setTokens = useCallback(
    (accessToken: string, refreshToken: string) => {
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
    throw new Error('useAuth must be used within AuthProvider');
  }
  return ctx;
}
