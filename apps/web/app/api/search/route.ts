import { createSearchAPI } from 'fumadocs-core/search/server';
import { getSource } from '@/lib/source';
export const dynamic = 'force-dynamic';
export async function GET(request: Request) {
  const query = new URL(request.url).searchParams;
  try {
    const space = query.get('space') ?? 'self';
    const context = await getSource(
      space,
      query.get('revision') ?? undefined,
      space === 'self',
    );
    const api = createSearchAPI('advanced', {
      indexes: context.source.getPages().map((page) => ({
        id: page.url,
        title: page.data.title,
        url: page.url,
        structuredData: page.data.structuredData,
      })),
    });
    const response = await api.GET(request);
    response.headers.set('X-Knowledge-Revision', context.scan.sourceRevision);
    response.headers.set('Cache-Control', 'no-store');
    response.headers.set('X-Knowledge-Stale', String(context.stale));
    return response;
  } catch {
    return Response.json({ error: '空间或知识修订不可用' }, { status: 404 });
  }
}
