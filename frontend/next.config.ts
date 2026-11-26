import type {NextConfig} from 'next';

const nextConfig: NextConfig = {
  output: 'standalone',
  async rewrites() {
    const origin = process.env.API_ORIGIN || 'http://127.0.0.1:3123';
    return [
      {source: '/api/:path*', destination: `${origin}/api/:path*`},
      {source: '/swagger/:path*', destination: `${origin}/swagger/:path*`},
    ];
  },
};

export default nextConfig;
