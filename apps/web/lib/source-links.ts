import path from 'node:path';

interface LinkDocument {
  sourceId: string;
  documentId: string;
  relativePath: string;
}

export function documentUrl(
  document: LinkDocument,
  baseUrl: string,
  revision?: string,
): string {
  const suffix =
    baseUrl === '/read' &&
    document.sourceId === 'knowledge' &&
    document.documentId === 'index'
      ? ''
      : baseUrl === '/read' && document.sourceId === 'knowledge'
        ? `/${encodeURIComponent(document.documentId)}`
        : `/${encodeURIComponent(document.sourceId)}/${encodeURIComponent(document.documentId)}`;
  return `${baseUrl}${suffix}${revision ? `?revision=${encodeURIComponent(revision)}` : ''}`;
}

export function resolveDocumentLink(
  href: string,
  current: LinkDocument,
  documents: LinkDocument[],
  baseUrl: string,
  revision: string,
  sourceRoots?: Record<string, string>,
): string | undefined {
  if (/^(https?:|mailto:|#)/i.test(href)) {
    return href;
  }
  if (href.startsWith('/') || /^[a-z][a-z\d+.-]*:/i.test(href)) {
    return undefined;
  }
  const [pathname, hash] = href.split('#', 2);
  let decoded: string;
  try {
    decoded = decodeURIComponent(pathname.split('?')[0]);
  } catch {
    return undefined;
  }
  const target = path.posix.normalize(
    path.posix.join(path.posix.dirname(current.relativePath), decoded),
  );
  const root = sourceRoots?.[current.sourceId];
  const absolute = root
    ? path.resolve(root, path.posix.dirname(current.relativePath), decoded)
    : undefined;
  const crossSource =
    absolute &&
    documents.find(
      (item) =>
        sourceRoots?.[item.sourceId] &&
        path.resolve(sourceRoots[item.sourceId], item.relativePath) ===
          absolute,
    );
  if (target.startsWith('../') && !crossSource) {
    return undefined;
  }
  const document =
    crossSource ||
    documents.find(
      (item) =>
        item.sourceId === current.sourceId && item.relativePath === target,
    );
  return document
    ? `${documentUrl(document, baseUrl, revision)}${hash ? `#${hash}` : ''}`
    : undefined;
}

export function resolveAttachmentPath(
  href: string,
  documentPath: string,
): string | undefined {
  if (href.startsWith('/') || /^[a-z][a-z\d+.-]*:/i.test(href)) {
    return undefined;
  }
  let decoded: string;
  try {
    decoded = decodeURIComponent(href.split(/[?#]/)[0]);
  } catch {
    return undefined;
  }
  const target = path.posix.normalize(
    path.posix.join(path.posix.dirname(documentPath), decoded),
  );
  if (
    target.startsWith('../') ||
    !/\.(png|jpe?g|gif|webp|avif)$/i.test(target)
  ) {
    return undefined;
  }
  return target;
}
