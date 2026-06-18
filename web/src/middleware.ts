/** 路由守卫 — JWT 认证 + RBAC + Token 自动续期。 */

import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

const PUBLIC_PATHS = ['/login'];
const ADMIN_PATH = '/admin';
const ADMIN_ROLES = ['系统管理员', 'admin', 'operator', 'knowledge_manager'];

interface JwtPayload {
  roles?: string[];
  exp?: number;
  [key: string]: unknown;
}

function decodePayload(token: string): JwtPayload | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    return JSON.parse(atob(base64));
  } catch {
    return null;
  }
}

function isExpired(token: string, bufferMs = 60_000): boolean {
  const p = decodePayload(token);
  if (!p?.exp) return true;
  return p.exp * 1000 < Date.now() + bufferMs;
}

async function refreshAccessToken(refreshToken: string, requestUrl: string): Promise<string | null> {
  try {
    const apiUrl = new URL('/api/v1/auth/refresh', requestUrl);
    const res = await fetch(apiUrl.toString(), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    if (!res.ok) return null;
    const json = await res.json();
    return json?.data?.access_token || null;
  } catch {
    return null;
  }
}

export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const accessToken = request.cookies.get('access_token')?.value;
  const refreshToken = request.cookies.get('refresh_token')?.value;

  if (PUBLIC_PATHS.includes(pathname)) {
    if (accessToken && !isExpired(accessToken)) {
      return NextResponse.redirect(new URL('/portal/chat', request.url));
    }
    return NextResponse.next();
  }

  if (pathname.startsWith('/_next') || pathname.startsWith('/favicon') || pathname.match(/\.(svg|png|jpg|css)$/)) {
    return NextResponse.next();
  }

  if (!accessToken) {
    return NextResponse.redirect(new URL('/login', request.url));
  }

  // Token 过期 → 尝试 refresh 自动续期
  if (isExpired(accessToken)) {
    if (refreshToken) {
      const newToken = await refreshAccessToken(refreshToken, request.url);
      if (newToken) {
        const response = NextResponse.next();
        response.cookies.set('access_token', newToken, { path: '/', httpOnly: false, sameSite: 'lax' });
        return response;
      }
    }
    const response = NextResponse.redirect(new URL('/login', request.url));
    response.cookies.delete('access_token');
    response.cookies.delete('refresh_token');
    return response;
  }

  // RBAC
  if (pathname.startsWith(ADMIN_PATH)) {
    const payload = decodePayload(accessToken);
    if (!payload?.roles?.some((r) => ADMIN_ROLES.includes(r))) {
      return NextResponse.redirect(new URL('/portal/chat', request.url));
    }
  }

  return NextResponse.next();
}

export const config = {
  matcher: ['/((?!api|_next/static|_next/image).*)'],
};
