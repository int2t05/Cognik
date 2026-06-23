import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  output: 'standalone',
  // 显式指定 turbopack 工作区根目录，避免向上遍历检测到 C:\Users\int2t\package-lock.json
  // process.cwd() 返回运行时的绝对路径（npm run dev/build 在 web/ 下执行，CWD 即为 web/）
  turbopack: { root: process.cwd() },
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
