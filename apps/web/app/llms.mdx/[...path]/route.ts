import { getSource } from '@/lib/source';
export const dynamic = 'force-dynamic';
export async function GET(
  request: Request,
  { params }: RouteContext<'/llms.mdx/[...path]'>,
) {
  const { path } = await params;
  const query = new URL(request.url).searchParams;
  const legacy = path[0] === 'read' || path[0] === 'read.md';
  const space = legacy
    ? 'self'
    : path[0] === 'spaces' && path[2] === 'read'
      ? path[1]
      : undefined;
  if (!space) {
    return new Response(null, { status: 404 });
  }
  try {
    const context = await getSource(
      space,
      query.get('revision') ?? undefined,
      legacy,
    );
    const segments = (legacy ? path.slice(1) : path.slice(3)).map((item) =>
      item.replace(/\.(md|txt)$/, ''),
    );
    const page = context.source.getPage(segments.length ? segments : undefined);
    if (!page) {
      return new Response(null, { status: 404 });
    }
    return new Response(await context.text.page(page), {
      headers: {
        'Content-Type': 'text/markdown; charset=utf-8',
        'Cache-Control': 'no-store',
        'X-Knowledge-Revision': context.scan.sourceRevision,
        'X-Knowledge-Stale': String(context.stale),
      },
    });
  } catch {
    return new Response(null, { status: 404 });
  }
}
