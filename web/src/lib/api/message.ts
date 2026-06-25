import { apiFetch, apiFetchPage } from './client';
import { PAGE_SIZE } from './constants';

export interface MessageItem { id: number; user_id: number; title: string; content: string; type: string; related_type: string; related_id: number; is_read: boolean; created_at: string; }

/** 消息模块 API 路径，供 useAccountSwitcher 等模块复用，避免硬编码漂移。 */
export const MESSAGE_PATHS = {
  list: '/api/v1/portal/messages',
  unreadCount: '/api/v1/portal/messages/unread-count',
  readAll: '/api/v1/portal/messages/read-all',
  markRead: (id: number) => `/api/v1/portal/messages/${id}/read`,
} as const;

export function getMessages(page: number) { return apiFetchPage<MessageItem>(`${MESSAGE_PATHS.list}?page=${page}&page_size=${PAGE_SIZE}`); }
export function markAsRead(id: number) { return apiFetch<{ unread_count: number }>(MESSAGE_PATHS.markRead(id), { method: 'PUT' }); }
export function markAllRead() { return apiFetch<{ affected: number }>(MESSAGE_PATHS.readAll, { method: 'PUT' }); }

/** getUnreadCount 查询未读消息数。可传入可选 token 用于跨账号验证（useAccountSwitcher）。 */
export async function getUnreadCount(token?: string): Promise<{ count: number }> {
  if (!token) return apiFetch<{ count: number }>(MESSAGE_PATHS.unreadCount);
  const BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
  const res = await fetch(`${BASE}${MESSAGE_PATHS.unreadCount}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const json = await res.json() as Record<string, unknown>;
  if (json.code !== 0) {
    throw { code: json.code as number, message: json.message as string };
  }
  return json.data as { count: number };
}
