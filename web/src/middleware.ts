/**
 * 路由守卫 — JWT 认证 + RBAC + base64url 兼容 Token 解码。
 *
 * 修复旧版 atob 不兼容 base64url 导致 JWT 过期检查失效的问题。
 * 参照 docs/TODO.md P0-1。
 */

import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

// 公开路由（无需认证）
const PUBLIC_PATHS = ['/login'];

// 需要 RBAC 检查的路由
const ADMIN_PATH = '/admin';

interface JwtPayload {
  roles?: string[];
  exp?: number;
  [key: string]: unknown;
}

function decodePayload(token: string): JwtPayload | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;
    // base64url → base64（修复 P0-1）
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    return JSON.parse(atob(base64));
  } catch {
    return null;
  }
}

function isExpired(token: string): boolean {
  const p = decodePayload(token);
  if (!p?.exp) return true;
  return p.exp * 1000 < Date.now() + 60_000; // 60s 缓冲
}

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const token = request.cookies.get('access_token')?.value;

  // 公开路由
  if (PUBLIC_PATHS.includes(pathname)) {
    if (token && !isExpired(token)) {
      return NextResponse.redirect(new URL('/portal/chat', request.url));
    }
    return NextResponse.next();
  }

  // 静态资源
  if (
    pathname.startsWith('/_next') ||
    pathname.startsWith('/favicon') ||
    pathname.match(/\.(svg|png|jpg|css)$/)
  ) {
    return NextResponse.next();
  }

  // 认证检查
  if (!token) {
    return NextResponse.redirect(new URL('/login', request.url));
  }

  if (isExpired(token)) {
    const response = NextResponse.redirect(new URL('/login', request.url));
    response.cookies.delete('access_token');
    return response;
  }

  // RBAC：后台路由需要 admin 相关角色
  if (pathname.startsWith(ADMIN_PATH)) {
    const payload = decodePayload(token);
    const roles = payload?.roles || [];
    const adminRoles = ['系统管理员', 'admin', 'operator', 'knowledge_manager'];
    if (!roles.some((r) => adminRoles.includes(r))) {
      return NextResponse.redirect(new URL('/portal/chat', request.url));
    }
  }

  return NextResponse.next();
}

export const config = {
  matcher: ['/((?!api|_next/static|_next/image).*)'],
};
