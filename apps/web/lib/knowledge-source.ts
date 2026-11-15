import { loader, type Source, type MetaData } from 'fumadocs-core/source';
import { structure, type StructuredData } from 'fumadocs-core/mdx-plugins';
import type {
  KnowledgeDocument,
  ScanResult,
} from '@ai2workspace/core/knowledge';
import type { SourceDefinition } from '@ai2workspace/core/workspace';
import { documentUrl } from './source-links.ts';

type ReadingSource = Source<{
  pageData: {
    title: string;
    document: KnowledgeDocument;
    structuredData: StructuredData;
    getText: () => Promise<string>;
  };
  metaData: MetaData;
}>;

export function buildKnowledgeSource(
  scan: ScanResult,
  definitions: SourceDefinition[],
  baseUrl: string,
  legacy = false,
) {
  const sources = scan.sources ?? [
    ...definitions,
    ...[...new Set(scan.documents.map((d) => d.sourceId))]
      .filter((id) => !definitions.some((s) => s.id === id))
      .map((id) => ({ id, kind: 'reference' as const })),
  ];
  const knowledge = sources.filter((s) => s.kind === 'knowledge');
  const main = knowledge.length === 1 ? knowledge[0]?.id : undefined;
  const prefix = (sourceId: string) => `${sourceId}/`;
  const files: ReadingSource['files'] = scan.documents.map((document) => ({
    type: 'page' as const,
    path: `${prefix(document.sourceId)}${document.relativePath}`,
    slugs:
      legacy && document.sourceId === 'knowledge'
        ? document.documentId === 'index'
          ? []
          : [document.documentId]
        : [document.sourceId, document.documentId],
    data: {
      title: document.title,
      document,
      structuredData: structure(document.body),
      getText: async () => document.body,
    },
  }));
  for (const metadata of scan.navigation ?? []) {
    const { sourceId, relativePath, ...data } = metadata;
    files.push({
      type: 'meta',
      path: `${prefix(sourceId)}${relativePath}`,
      data,
    });
  }
  const groups = sources.filter(
    (s) => s.id !== main && scan.documents.some((d) => d.sourceId === s.id),
  );
  for (const source of groups) {
    const metaPath = `${source.id}/meta.json`;
    const existing = files.find(
      (f) => f.type === 'meta' && f.path === metaPath,
    );
    if (!existing) {
      files.push({
        type: 'meta',
        path: metaPath,
        data: {
          title: source.kind === 'instruction' ? '适用规范' : source.id,
          defaultOpen: source.kind !== 'instruction',
        },
      });
    }
  }
  files.push({
    type: 'meta',
    path: 'meta.json',
    data: {
      pages: [
        ...(main ? [main] : []),
        ...groups.filter((s) => s.id !== main).map((s) => s.id),
      ],
    },
  });
  return loader(
    { files },
    {
      baseUrl,
      pageTree: {
        transformers: [
          {
            root(node) {
              return {
                ...node,
                children: node.children.flatMap((child) =>
                  main && child.type === 'folder' && child.$ref?.folder === main
                    ? [...(child.index ? [child.index] : []), ...child.children]
                    : [child],
                ),
              };
            },
          },
        ],
      },
      url(slugs) {
        const document = scan.documents.find((item) =>
          legacy && slugs.length < 2
            ? item.sourceId === 'knowledge' &&
              item.documentId === (slugs[0] ?? 'index')
            : item.sourceId === slugs[0] && item.documentId === slugs[1],
        );
        return document
          ? documentUrl(document, baseUrl, scan.sourceRevision)
          : baseUrl;
      },
    },
  );
}
