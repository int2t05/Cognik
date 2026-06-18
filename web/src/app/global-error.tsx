'use client';

import { AppleButton } from '@/components/ui/AppleButton';
import s from './global-error.module.css';

export default function GlobalError({ error, reset }: { error: Error; reset: () => void }) {
  return (
    <html lang="zh-CN">
      <body className={s.body}>
        <div className={s.wrapper}>
          <div className={s.inner}>
            <h1 className={s.title}>系统错误</h1>
            <p className={s.message}>{error.message}</p>
            <AppleButton onClick={reset}>重试</AppleButton>
          </div>
        </div>
      </body>
    </html>
  );
}
