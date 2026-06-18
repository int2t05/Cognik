'use client';

import { AppleButton } from '@/components/ui/AppleButton';

export default function GlobalError({ error, reset }: { error: Error; reset: () => void }) {
  return (
    <html lang="zh-CN">
      <body className="m-0 font-sans">
        <div className="min-h-screen flex items-center justify-center bg-[#f5f5f7]">
          <div className="text-center max-w-[400px]">
            <h1 className="text-[34px] font-semibold text-[#1d1d1f] mb-3">系统错误</h1>
            <p className="text-[15px] text-[#7a7a7a] mb-6">{error.message}</p>
            <AppleButton onClick={reset}>重试</AppleButton>
          </div>
        </div>
      </body>
    </html>
  );
}
