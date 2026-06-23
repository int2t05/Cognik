import { apiFetch } from './client';

/** 修改当前用户密码。login 和 refreshToken 逻辑分别在 login/page.tsx 和 proxy.ts 中实现。 */
export function changePassword(old_password: string, new_password: string) {
  return apiFetch<null>('/api/v1/auth/me/change-password', { method: 'POST', body: JSON.stringify({ old_password, new_password }) });
}
