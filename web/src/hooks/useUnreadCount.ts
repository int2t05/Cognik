/**
 * useUnreadCount — 消息未读数轮询 hook。
 *
 * AdminLayout 和 PortalLayout 之前各自实现了完全相同的轮询逻辑，
 * 提取为共享 hook 以消除重复。
 * 默认每 30 秒轮询一次。
 */

import { useState, useEffect, useCallback } from 'react';
import { getUnreadCount } from '@/lib/api/message';

export function useUnreadCount(interval = 30000) {
  const [unreadCount, setUnreadCount] = useState(0);

  const refresh = useCallback(() => {
    getUnreadCount()
      .then((d) => setUnreadCount(d.count))
      .catch((err: unknown) => {
        console.warn('获取未读数失败:', err);
      });
  }, []);

  useEffect(() => {
    refresh();
    const t = setInterval(refresh, interval);
    return () => clearInterval(t);
  }, [refresh, interval]);

  return { unreadCount, refresh };
}
