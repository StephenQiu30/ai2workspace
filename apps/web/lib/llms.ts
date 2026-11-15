import { getSource } from './source';
export async function serveLlms(request: Request, full: boolean) {
  const params = new URL(request.url).searchParams;
  const space = params.get('space') ?? 'self';
  try {
    const context = await getSource(
      space,
      params.get('revision') ?? undefined,
      space === 'self',
    );
    const content = full
      ? await context.text.full()
      : await context.text.index();
    const text = `<!-- ai2workspace; workspace: ${context.workspace.definition.id}; revision: ${context.scan.sourceRevision}; stale: ${context.stale} -->\n\n${content}`;
    return new Response(text, {
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
