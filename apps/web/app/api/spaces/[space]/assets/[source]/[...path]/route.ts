import { openSpaceStore } from '@ai2workspace/core/workspace';
import { readIndexedAsset } from '@ai2workspace/core/knowledge';
import { resolveRegisteredWorkspace } from '@/lib/workspaces';
export const dynamic = 'force-dynamic';

export async function GET(
  request: Request,
  { params }: RouteContext<'/api/spaces/[space]/assets/[source]/[...path]'>,
) {
  const revision = new URL(request.url).searchParams.get('revision');
  if (!revision || !/^[a-f0-9]{64}$/.test(revision)) {
    return new Response(null, { status: 404 });
  }
  try {
    const { space, source, path } = await params;
    const workspace = await resolveRegisteredWorkspace(space);
    const store = await openSpaceStore(workspace);
    const image = await readIndexedAsset(
      workspace,
      store,
      revision,
      source,
      path.join('/'),
    );
    return new Response(new Uint8Array(image.bytes), {
      headers: {
        'Content-Type': image.mimeType,
        'X-Content-Type-Options': 'nosniff',
        'Cache-Control': 'private, max-age=31536000, immutable',
        'Content-Security-Policy': "default-src 'none'; sandbox",
        'X-Knowledge-Revision': revision,
        ETag: `"${image.hash}"`,
      },
    });
  } catch {
    return new Response(null, { status: 404 });
  }
}
