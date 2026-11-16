const config = {
  reactStrictMode: true,
  experimental: { cpus: 2 },
  serverExternalPackages: ['@ai2workspace/core'],
  images: { localPatterns: [{ pathname: '/api/spaces/**/assets/**' }] },
  // Spaces, source content and local runtime data are supplied at runtime.
  outputFileTracingExcludes: {
    '/*': [
      '../../spaces/**',
      '../../knowledge/**',
      '../../src/content/**',
      '../../harness/**',
      '../../code/**',
      '../../**/.local/**',
      '**/workspace.local.yaml',
      '**/.env',
      '**/.env.*',
    ],
  },
  async rewrites() {
    return [
      { source: '/read.md', destination: '/llms.mdx/read.md' },
      { source: '/read/:path*.txt', destination: '/llms.mdx/read/:path*.txt' },
      { source: '/read/:path*.md', destination: '/llms.mdx/read/:path*.md' },
      {
        source: '/spaces/:space/read/:path*.md',
        destination: '/llms.mdx/spaces/:space/read/:path*.md',
      },
    ];
  },
};

export default config;
