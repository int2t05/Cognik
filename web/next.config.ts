import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  output: 'standalone',
  turbopack: { root: process.cwd() },
  async headers() {
    return [{
      source: '/:path*',
      headers: [{ key: 'Referrer-Policy', value: 'no-referrer' }],
    }];
  },
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
