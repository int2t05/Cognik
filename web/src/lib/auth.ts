/** JWT 工具 — base64url 兼容解码。 */

export interface JwtPayload {
  sub?: string;
  roles?: string[];
  exp?: number;
  iat?: number;
  [key: string]: unknown;
}

/** 解码 JWT payload（不验证签名），兼容 base64url，正确处理 UTF-8 多字节字符。 */
export function decodeJwtPayload(token: string): JwtPayload | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;

    const payload = parts[1];
    // base64url → base64：替换非标准字符
    const base64 = payload.replace(/-/g, '+').replace(/_/g, '/');
    // atob 返回的二进制字符串每个字符对应一个字节（Latin-1 范围），
    // 中文等多字节 UTF-8 字符会被拆分，需要通过 TextDecoder 重新解码。
    const binary = atob(base64);
    const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0));
    const json = new TextDecoder().decode(bytes);
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
