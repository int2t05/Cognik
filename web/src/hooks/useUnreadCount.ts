/** useUnreadCount 消息未读数轮询 hook，SWR 全局缓存去重，30s 刷新。 */

import useSWR from 'swr';
import { getUnreadCount } from '@/lib/api/message';

export function useUnreadCount() {
  const { data } = useSWR('unread-count', () => getUnreadCount(), {
    refreshInterval: 30000,
    dedupingInterval: 5000,
  });

  return { unreadCount: data?.count ?? 0 };
}
