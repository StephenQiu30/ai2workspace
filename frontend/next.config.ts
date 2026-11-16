import type {NextConfig} from 'next';

const nextConfig: NextConfig = {
  output: 'standalone',
  async rewrites() {
    const origin = process.env.API_ORIGIN;
    return origin
      ? [{source: '/api/:path*', destination: `${origin}/api/:path*`}]
      : [];
  },
};

export default nextConfig;
