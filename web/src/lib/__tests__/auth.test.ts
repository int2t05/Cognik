import { describe, it, expect } from 'vitest';
import { decodeJwtPayload, isTokenExpired } from '../auth';

// 使用 base64url 编码构造测试 JWT token
const VALID_TOKEN =
  'eyJhbGciOiJIUzI1NiJ9.' +
  btoa(JSON.stringify({ sub: '1', roles: ['admin'], exp: 9999999999, iat: 1 }))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '') +
  '.fake-sig';

describe('decodeJwtPayload', () => {
  it('正确解码标准 base64url JWT payload', () => {
    const payload = decodeJwtPayload(VALID_TOKEN);
    expect(payload).not.toBeNull();
    expect(payload!.sub).toBe('1');
    expect(payload!.roles).toEqual(['admin']);
  });

  it('返回 null 当 token 格式无效（缺少点号分隔）', () => {
    expect(decodeJwtPayload('not-a-jwt')).toBeNull();
  });

  it('返回 null 当 payload 无法 JSON 解析', () => {
    const badToken = 'header.' + btoa('not-json').replace(/\+/g, '-').replace(/\//g, '_') + '.sig';
    expect(decodeJwtPayload(badToken)).toBeNull();
  });
});

describe('isTokenExpired', () => {
  it('返回 false 当 token 未过期（含 60s 缓冲）', () => {
    const futureExp = Math.floor(Date.now() / 1000) + 600; // 10 分钟后
    const token =
      'header.' +
      btoa(JSON.stringify({ exp: futureExp }))
        .replace(/\+/g, '-')
        .replace(/\//g, '_')
        .replace(/=+$/, '') +
      '.sig';
    expect(isTokenExpired(token)).toBe(false);
  });

  it('返回 true 当 token 已过期', () => {
    const pastExp = Math.floor(Date.now() / 1000) - 120; // 2 分钟前
    const token =
      'header.' +
      btoa(JSON.stringify({ exp: pastExp }))
        .replace(/\+/g, '-')
        .replace(/\//g, '_')
        .replace(/=+$/, '') +
      '.sig';
    expect(isTokenExpired(token)).toBe(true);
  });

  it('返回 true 当 token 无 exp 字段', () => {
    const token =
      'header.' +
      btoa(JSON.stringify({ sub: '1' }))
        .replace(/\+/g, '-')
        .replace(/\//g, '_')
        .replace(/=+$/, '') +
      '.sig';
    expect(isTokenExpired(token)).toBe(true);
  });
});
