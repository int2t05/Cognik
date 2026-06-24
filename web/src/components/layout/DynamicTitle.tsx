/**
 * DynamicTitle — 客户端挂载后从公开端点读取 app_name 并更新 document.title。
 *
 * SSR 阶段使用默认值（避免 hydration 闪烁），客户端获取到实际值后原地替换。
 */
'use client';

import { useEffect } from 'react';
import useSWR from 'swr';
import { getPublicConfig } from '@/lib/api/config';
import { getAppName } from '@/lib/config/defaults';

export function DynamicTitle() {
  const { data } = useSWR('public-app-name-title', () => getPublicConfig('app_name'), {
    revalidateOnFocus: true,
    dedupingInterval: 30_000,
  });

  useEffect(() => {
    const name = typeof data === 'string' ? data : getAppName();
    document.title = `${name} — 运维数字员工`;
  }, [data]);

  return null;
}
