import { NextRequest, NextResponse } from 'next/server';
import { isMarkdownPreferred } from 'fumadocs-core/negotiation';

export default function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const headers = new Headers(request.headers);
  headers.delete('x-ai2workspace-revision');
  const revision = request.nextUrl.searchParams.get('revision');
  if (revision) {
    headers.set('x-ai2workspace-revision', revision);
  }
  if (
    (pathname === '/read' ||
      pathname.startsWith('/read/') ||
      /^\/spaces\/[^/]+\/read(?:\/|$)/.test(pathname)) &&
    isMarkdownPreferred(request) &&
    !/\.(md|txt)$/.test(pathname)
  ) {
    const url = request.nextUrl.clone();
    url.pathname = `/llms.mdx${pathname}.md`;
    return NextResponse.rewrite(url, {
      request: { headers },
      headers: { Vary: 'Accept' },
    });
  }
  return NextResponse.next({ request: { headers } });
}
