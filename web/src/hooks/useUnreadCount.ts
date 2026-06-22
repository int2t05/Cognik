/**
 * useUnreadCount — 消息未读数轮询 hook。
 *
 * 使用全局 SWR 缓存避免 AdminLayout + PortalLayout 同时挂载时的双轮询，
 * SWR 的 dedupingInterval 保证同一时间仅发一次请求。
 * 默认每 30 秒刷新一次。
 */

import useSWR from 'swr';
import { getUnreadCount } from '@/lib/api/message';

export function useUnreadCount() {
  const { data } = useSWR('unread-count', () => getUnreadCount(), {
    refreshInterval: 30000,
    dedupingInterval: 5000,
  });

  return { unreadCount: data?.count ?? 0 };
}
