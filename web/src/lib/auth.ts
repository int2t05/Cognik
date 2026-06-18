/**
 * JWT 工具 — base64url 兼容解码。
 *
 * 修复旧版 atob 不兼容 base64url（`-`/`_`）导致解码失败的问题。
 * 参照 docs/TODO.md P0-1。
 */

export interface JwtPayload {
  sub?: string;
  roles?: string[];
  exp?: number;
  iat?: number;
  [key: string]: unknown;
}

/** 解码 JWT payload（不验证签名），兼容 base64url */
export function decodeJwtPayload(token: string): JwtPayload | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;

    const payload = parts[1];
    // base64url → base64：替换非标准字符
    const base64 = payload.replace(/-/g, '+').replace(/_/g, '/');
    const json = atob(base64);
    return JSON.parse(json);
  } catch {
    return null;
  }
}

/** 检查 token 是否过期（含 60 秒缓冲） */
export function isTokenExpired(token: string): boolean {
  const payload = decodeJwtPayload(token);
  if (!payload || typeof payload.exp !== 'number') return true;
  // 60 秒缓冲，避免时钟偏差导致的边缘误判
  return payload.exp * 1000 < Date.now() + 60_000;
}
