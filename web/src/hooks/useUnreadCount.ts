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
      .catch(() => {
        /* 轮询失败静默处理——用户不应因未读数获取失败而受到打扰 */
      });
  }, []);

  useEffect(() => {
    refresh();
    const t = setInterval(refresh, interval);
    return () => clearInterval(t);
  }, [refresh, interval]);

  return { unreadCount, refresh };
}
