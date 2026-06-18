'use client';

import { AppleButton } from '@/components/ui/AppleButton';

export default function GlobalError({ error, reset }: { error: Error; reset: () => void }) {
  return (
    <html lang="zh-CN">
      <body style={{ margin: 0, fontFamily: 'system-ui, sans-serif' }}>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '100vh', background: '#f5f5f7' }}>
          <div style={{ textAlign: 'center', maxWidth: 400 }}>
            <h1 style={{ fontSize: 34, fontWeight: 600, color: '#1d1d1f', marginBottom: 12 }}>系统错误</h1>
            <p style={{ fontSize: 15, color: '#7a7a7a', marginBottom: 24 }}>{error.message}</p>
            <AppleButton onClick={reset}>重试</AppleButton>
          </div>
        </div>
      </body>
    </html>
  );
}
