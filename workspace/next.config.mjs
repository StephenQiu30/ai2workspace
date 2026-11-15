import { createMDX } from 'fumadocs-mdx/next';

const withMDX = createMDX();

export default withMDX({
  reactStrictMode: true,
  experimental: { cpus: 2 },
  async rewrites() {
    return [
      { source: '/read.md', destination: '/llms.mdx/read.md' },
      { source: '/read/:path*.md', destination: '/llms.mdx/read/:path*.md' },
      { source: '/read/:path*.txt', destination: '/llms.mdx/read/:path*.txt' },
    ];
  },
});
