import { apiFetch, apiFetchPage, ApiError } from './client';
import { PAGE_SIZE } from './constants';

export interface MessageItem { id: number; title: string; content: string; type: string; related_type: string; related_id: number; is_read: boolean; created_at: string; }

/** 消息模块 API 路径，供 useAccountSwitcher 等模块复用，避免硬编码漂移。 */
export const MESSAGE_PATHS = {
  list: '/api/v1/portal/messages',
  unreadCount: '/api/v1/portal/messages/unread-count',
  readAll: '/api/v1/portal/messages/read-all',
  markRead: (id: number) => `/api/v1/portal/messages/${id}/read`,
} as const;

export function getMessages(page: number, type?: string) {
  let url = `${MESSAGE_PATHS.list}?page=${page}&page_size=${PAGE_SIZE}`;
  if (type) url += `&type=${encodeURIComponent(type)}`;
  return apiFetchPage<MessageItem>(url);
}
export function markAsRead(id: number) { return apiFetch<{ unread_count: number }>(MESSAGE_PATHS.markRead(id), { method: 'PUT' }); }
export function markAllRead() { return apiFetch<{ affected: number }>(MESSAGE_PATHS.readAll, { method: 'PUT' }); }

/**
 * getUnreadCount 查询未读消息数。
 *
 * 传入 token 时使用原始 fetch 而非 apiFetch，原因是 rawApiRequest 会：
 * 1) 用 _tokenGetter() 覆盖 Authorization header（导致自定义 token 失效）；
 * 2) 捕获 code=10001 后自动清除认证并跳转登录页（useAccountSwitcher 需要自行处理 10001）。
 */
export async function getUnreadCount(token?: string): Promise<{ count: number }> {
  if (!token) return apiFetch<{ count: number }>(MESSAGE_PATHS.unreadCount);
  const BASE = process.env.NEXT_PUBLIC_API_URL || '';
  const res = await fetch(`${BASE}${MESSAGE_PATHS.unreadCount}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const json = await res.json() as Record<string, unknown>;
  if (json.code !== 0) {
    throw new ApiError(json.code as number, json.message as string);
  }
  return json.data as { count: number };
}
