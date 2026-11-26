import type {NextConfig} from 'next';

const LOCAL_SERVICE_PORTS: Record<string, number> = {
  identity: 3123,
  learning: 3124,
  commerce: 3125,
  community: 3126,
  assistant: 3127,
  studio: 3128,
};

const nextConfig: NextConfig = {
  output: 'standalone',
  async rewrites() {
    const origin = process.env.API_ORIGIN?.trim();
    const isSelfOrigin =
      !origin ||
      origin === 'http://127.0.0.1:3121' ||
      origin === 'http://localhost:3121';

    if (!isSelfOrigin) {
      return [{source: '/api/:path*', destination: `${origin}/api/:path*`}];
    }

    return Object.entries(LOCAL_SERVICE_PORTS).map(([service, port]) => ({
      source: `/api/${service}/:path*`,
      destination: `http://127.0.0.1:${port}/api/${service}/:path*`,
    }));
  },
};

export default nextConfig;
