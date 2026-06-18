/**
 * AuthContext — 全局认证状态管理。
 *
 * 替代旧版 Pinia stores/auth.ts，使用 React Context + localStorage 持久化。
 */

'use client';
import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  type ReactNode,
} from 'react';

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

  // 同步 token 到 cookie（供 middleware 读取）
  useEffect(() => {
    if (state.token) {
      document.cookie = `access_token=${state.token}; path=/; SameSite=Lax`;
    }
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
