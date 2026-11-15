import type { KnowledgeStore } from './ports.js';
import { realpathSync } from 'node:fs';
import { parseDocument } from 'yaml';
import { readFile, readdir } from 'node:fs/promises';
import path from 'node:path';
import { randomUUID } from 'node:crypto';
import { userInfo } from 'node:os';
import { z } from 'zod';
import type { ResolvedWorkspace } from '../workspace/types.js';
import type { Issue } from '../shared/types.js';
import type {
  ScanResult,
  KnowledgeDocument,
  Catalog,
  CatalogAnnotationInput,
  DocumentMetadata,
} from '../knowledge/types.js';
import {
  putObject,
  readObject,
  hash,
  digest,
  contained,
  atomicJson,
  maybeJson,
  fail,
} from '../storage/file-store.js';
export const privateNames =
  /^(?:\.env(?:\..*)?|.*\.(?:pem|key|p12|pfx)|id_(?:rsa|ed25519)|workspace\.local\.yaml)$/i;
const excludedDirs = new Set([
  'node_modules',
  '.git',
  '.next',
  '.source',
  '.local',
  'dist',
  'build',
  'coverage',
  'test-results',
  'playwright-report',
  '.test-build',
]);
export function excluded(rel: string, patterns: string[] = []): boolean {
  return (
    rel.split('/').some((x) => excludedDirs.has(x) || privateNames.test(x)) ||
    patterns.some((p) => path.matchesGlob(rel, p))
  );
}
export async function listFiles(
  root: string,
  patterns: string[] = [],
  issues: Issue[] = [],
  allowHidden = false,
): Promise<string[]> {
  const files: string[] = [];
  async function walk(dir: string) {
    for (const e of await readdir(path.join(root, dir), {
      withFileTypes: true,
    })) {
      const rel = path.posix.join(dir, e.name);
      if (excluded(rel, patterns)) {
        continue;
      }
      if (e.isSymbolicLink()) {
        issues.push({
          code: 'symlink_excluded',
          severity: 'warning',
          message: 'Symbolic link excluded',
          path: rel,
        });
        continue;
      }
      if (e.name.startsWith('.') && !allowHidden) {
        continue;
      }
      if (e.isDirectory()) {
        await walk(rel);
      } else if (e.isFile()) {
        files.push(rel);
      }
    }
  }
  await walk('');
  return files.sort();
}
export function documentKey(d: {
  workspaceId: string;
  sourceId: string;
  documentId: string;
}): string {
  return [d.workspaceId, d.sourceId, d.documentId].join('/');
}
function workspacePrivate(
  workspace: ResolvedWorkspace,
  root: string,
  rel: string,
): boolean {
  const target = path.resolve(root, rel);
  let dataDir = workspace.dataDir;
  try {
    dataDir = realpathSync.native(dataDir);
  } catch {
    /* A missing runtime directory has no publishable files. */
  }
  return (
    target === dataDir ||
    target.startsWith(dataDir + path.sep) ||
    target.startsWith(path.join(workspace.workspaceDir, 'tasks') + path.sep) ||
    ['catalog.json', 'workspace.local.yaml'].some(
      (p) => target === path.join(workspace.workspaceDir, p),
    )
  );
}
const imageTypes: Record<string, string> = {
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif': 'image/gif',
  '.webp': 'image/webp',
  '.avif': 'image/avif',
};
export async function scanUnlocked(
  workspace: ResolvedWorkspace,
  store: KnowledgeStore,
): Promise<ScanResult> {
  store.assertSpace(workspace);
  if (workspace.issues.some((i) => i.severity === 'error')) {
    fail(
      'source_unavailable',
      'Required source binding unavailable; previous complete index retained',
      3,
    );
  }
  const catalog = await store.readCatalog();
  if (catalog.workspaceId !== workspace.definition.id) {
    fail('workspace_mismatch', 'Catalog belongs to another space', 5);
  }
  const documents: KnowledgeDocument[] = [];
  const navigation: NonNullable<ScanResult['navigation']> = [];
  const issues: Issue[] = [...workspace.issues];
  const seen = new Set<string>();
  let changed = false;
  for (const source of workspace.definition.sources) {
    const root = workspace.bindings[source.binding];
    if (!root) {
      continue;
    }
    let imported: { id: string; path: string }[] = [];
    try {
      const v = await maybeJson<unknown>(path.join(root, 'document-ids.json'));
      if (Array.isArray(v)) {
        imported = v.filter(
          (x): x is { id: string; path: string } =>
            !!x && typeof x.id === 'string' && typeof x.path === 'string',
        );
      }
    } catch {
      issues.push({
        code: 'invalid_id_map',
        severity: 'error',
        message: 'Invalid document-ids.json',
        path: source.id,
      });
    }
    let files: string[];
    try {
      files = await listFiles(
        root,
        source.exclude,
        issues,
        source.kind === 'instruction' &&
          source.include.some((g) =>
            g.split('/').some((p) => p.startsWith('.') && p !== '.'),
          ),
      );
    } catch {
      fail(
        'source_unavailable',
        'Source became unavailable; previous complete index retained',
        3,
      );
    }
    if (source.kind !== 'instruction') {
      for (const rel of files.filter(
        (file) => path.posix.basename(file) === 'meta.json',
      )) {
        if (workspacePrivate(workspace, root, rel)) {
          continue;
        }
        const directory = path.posix.dirname(rel);
        const hasIncludedPage = files.some(
          (file) =>
            file.endsWith('.md') &&
            source.include.some((glob) => path.matchesGlob(file, glob)) &&
            (directory === '.' || file.startsWith(directory + '/')),
        );
        if (!hasIncludedPage) {
          continue;
        }
        try {
          const metadata = z
            .object({
              title: z.string().max(200).optional(),
              pages: z
                .array(
                  z
                    .string()
                    .max(200)
                    .refine(
                      (entry) =>
                        !entry.includes('/') &&
                        !entry.includes('\\') &&
                        entry !== '..',
                    ),
                )
                .max(1000)
                .optional(),
              defaultOpen: z.boolean().optional(),
            })
            .parse(
              JSON.parse(await readFile(await contained(root, rel), 'utf8')),
            );
          navigation.push({
            sourceId: source.id,
            relativePath: rel,
            ...metadata,
          });
        } catch {
          issues.push({
            code: 'invalid_navigation',
            severity: 'warning',
            message:
              'Directory navigation metadata is invalid; inferred directory navigation remains available',
            path: rel,
          });
        }
      }
    }
    for (const rel of files) {
      if (
        workspacePrivate(workspace, root, rel) ||
        !rel.endsWith('.md') ||
        !source.include.some((g) => path.matchesGlob(rel, g))
      ) {
        continue;
      }
      try {
        const body = await readFile(await contained(root, rel), 'utf8');
        const bodyHash = hash(body);
        const frontmatter = /^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/.exec(body);
        let bodyMetadata: Record<string, unknown> = {};
        if (frontmatter) {
          try {
            const parsed = parseDocument(frontmatter[1]!, { uniqueKeys: true });
            if (parsed.errors.length) {
              throw Error();
            }
            const value: unknown = parsed.toJS({ maxAliasCount: 0 });
            if (value && typeof value === 'object' && !Array.isArray(value)) {
              bodyMetadata = value as Record<string, unknown>;
            }
          } catch {
            issues.push({
              code: 'invalid_frontmatter',
              severity: 'warning',
              message: 'Frontmatter could not be parsed safely',
              path: rel,
            });
          }
        }
        let entry = catalog.documents.find(
          (d) => d.sourceId === source.id && d.relativePath === rel,
        );
        if (!entry) {
          const legacy = imported.find((i) => i.path === rel)?.id;
          const collision = legacy
            ? catalog.documents.find(
                (d) => d.sourceId === source.id && d.documentId === legacy,
              )
            : undefined;
          if (collision) {
            if (files.includes(collision.relativePath)) {
              issues.push({
                code: 'identity_conflict',
                severity: 'error',
                message:
                  'Imported identity already exists at another live path',
                path: rel,
              });
              continue;
            }
            entry = collision;
            entry.relativePath = rel;
            changed = true;
          }
          const possible = catalog.documents.filter(
            (d) =>
              d.sourceId === source.id &&
              d.hash === bodyHash &&
              d.relativePath !== rel,
          );
          if (possible.length) {
            issues.push({
              code: 'ambiguous_move',
              severity: 'warning',
              message:
                'Same body exists under another identity; explicit mapping required',
              path: rel,
            });
          }
          if (!entry) {
            entry = {
              workspaceId: workspace.definition.id,
              sourceId: source.id,
              documentId: legacy || randomUUID(),
              relativePath: rel,
              hash: bodyHash,
              metadata: { status: 'unclassified' },
            };
            catalog.documents.push(entry);
          }
          changed = true;
        }
        if (entry.hash !== bodyHash) {
          entry.history ??= [];
          entry.history.push({
            hash: entry.hash,
            metadata: { ...entry.metadata },
          });
          if (
            entry.metadata.status === 'confirmed' ||
            entry.metadata.status === 'superseded'
          ) {
            entry.metadata = { ...entry.metadata, status: 'draft' };
          }
          entry.hash = bodyHash;
          changed = true;
        }
        if (entry.missing) {
          entry.missing = false;
          changed = true;
        }
        seen.add(documentKey(entry));
        for (const field of ['status', 'type', 'domain', 'source'] as const) {
          if (
            bodyMetadata[field] !== undefined &&
            bodyMetadata[field] !== entry.metadata[field] &&
            (field === 'status' || entry.metadata[field] !== undefined)
          ) {
            issues.push({
              code: 'metadata_conflict',
              severity: 'warning',
              message: 'Body metadata conflicts with authoritative sidecar',
              path: rel,
              documentId: entry.documentId,
            });
          }
        }
        const links = [
          ...body.matchAll(/!?\[[^\]]*\]\(([^\s)]+)(?:\s+[^)]*)?\)/g),
        ]
          .map((m) => m[1]!)
          .filter(Boolean);
        const title =
          /^#\s+(.+)$/m.exec(body)?.[1] ?? path.basename(rel, '.md');
        documents.push({ ...entry, title, body, links });
        if (entry.metadata.status === 'unclassified') {
          issues.push({
            code: 'unclassified',
            severity: 'info',
            message: 'Document has no governance classification',
            path: rel,
            documentId: entry.documentId,
          });
        }
        for (const link of links) {
          if (/^(?:https?:|mailto:|#)/i.test(link)) {
            continue;
          }
          let target: string;
          try {
            target = decodeURIComponent(link.split('#')[0]!);
            if (!target) {
              continue;
            }
            const dest = path.posix.normalize(
              path.posix.join(path.posix.dirname(rel), target),
            );
            if (
              excluded(dest) ||
              dest.startsWith('../') ||
              path.isAbsolute(dest)
            ) {
              throw Error('outside');
            }
            await contained(root, dest);
          } catch {
            issues.push({
              code: 'broken_link',
              severity: 'warning',
              message: 'Local reference is missing or not permitted',
              path: rel,
            });
          }
        }
      } catch {
        issues.push({
          code: 'unreadable_source',
          severity: 'error',
          message: 'Document could not be safely read',
          path: rel,
        });
      }
    }
  }
  for (const entry of catalog.documents) {
    if (!seen.has(documentKey(entry)) && !entry.missing) {
      entry.missing = true;
      changed = true;
    }
  }
  const assets: NonNullable<ScanResult['assets']> = [];
  for (const doc of documents) {
    const source = workspace.definition.sources.find(
      (s) => s.id === doc.sourceId,
    )!;
    const root = workspace.bindings[source.binding]!;
    for (const link of doc.links) {
      if (/^(?:[a-z]+:|#)/i.test(link)) {
        continue;
      }
      try {
        const rel = path.posix.normalize(
          path.posix.join(
            path.posix.dirname(doc.relativePath),
            decodeURIComponent(link.split('#')[0]!),
          ),
        );
        const mimeType = imageTypes[path.extname(rel).toLowerCase()];
        if (
          !mimeType ||
          excluded(rel, source.exclude) ||
          workspacePrivate(workspace, root, rel) ||
          rel.split('/').some((p) => p.startsWith('.'))
        ) {
          continue;
        }
        if (
          assets.some((a) => a.sourceId === source.id && a.relativePath === rel)
        ) {
          continue;
        }
        const bytes = await readFile(await contained(root, rel));
        assets.push({
          sourceId: source.id,
          relativePath: rel,
          hash: hash(bytes),
          objectPath: await putObject(store, bytes),
          mimeType,
        });
      } catch {
        issues.push({
          code: 'unavailable_asset',
          severity: 'warning',
          message: 'Local image asset could not be captured',
          path: doc.relativePath,
        });
      }
    }
  }
  if (changed) {
    catalog.revision++;
    await store.writeCatalog(catalog);
  }
  const sources = workspace.definition.sources.map(({ id, kind }) => ({
    id,
    kind,
  }));
  const sourceRevision = digest({
    documents,
    issues,
    assets,
    navigation,
    sources,
  });
  const result: ScanResult = {
    workspaceId: workspace.definition.id,
    sourceRevision,
    sources,
    navigation,
    documents,
    issues,
    assets,
  };
  await atomicJson(
    path.join(store.root, 'indexes', sourceRevision + '.json'),
    result,
    true,
  );
  await atomicJson(path.join(store.root, 'indexes', 'current.json'), {
    revision: sourceRevision,
  });
  return result;
}
export async function scanSources(
  workspace: ResolvedWorkspace,
  store: KnowledgeStore,
): Promise<ScanResult> {
  store.assertSpace(workspace);
  return store.withLock(() => scanUnlocked(workspace, store));
}
export async function annotateCatalog(
  workspace: ResolvedWorkspace,
  store: KnowledgeStore,
  input: CatalogAnnotationInput,
): Promise<Catalog> {
  store.assertSpace(workspace);
  const key = z
    .strictObject({
      schemaVersion: z.literal(1),
      workspaceId: z.string(),
      sourceId: z.string(),
      documentId: z.string(),
      expectedRevision: z.number().int().min(0),
      fields: z.strictObject({
        status: z
          .enum(['unclassified', 'draft', 'confirmed', 'superseded'])
          .optional(),
        type: z.string().optional(),
        domain: z.string().optional(),
        projectIds: z.array(z.string()).optional(),
        source: z.string().optional(),
        dependencies: z
          .array(
            z.strictObject({
              workspaceId: z.string(),
              sourceId: z.string(),
              documentId: z.string(),
            }),
          )
          .optional(),
      }),
      reason: z.string().min(1),
      decisionRef: z.string().min(1).optional(),
    })
    .safeParse(input);
  if (!key.success) {
    fail('invalid_annotation', key.error.message);
  }
  if (input.workspaceId !== workspace.definition.id) {
    fail('workspace_mismatch', 'Annotation belongs to another space', 5);
  }
  return store.withLock(async () => {
    const catalog = await store.readCatalog();
    if (catalog.revision !== input.expectedRevision) {
      fail('revision_conflict', 'Catalog revision changed', 5);
    }
    const entry = catalog.documents.find(
      (d) => documentKey(d) === documentKey(input),
    );
    if (!entry || entry.missing) {
      fail('missing_document', 'Annotation document unavailable', 3);
    }
    if (
      ['confirmed', 'superseded'].includes(input.fields.status ?? '') &&
      !input.decisionRef
    ) {
      fail(
        'missing_decision',
        'Confirmation requires an explicit decision reference',
      );
    }
    const metadata: DocumentMetadata = { ...entry.metadata, ...input.fields };
    if (
      input.fields.status === 'confirmed' ||
      input.fields.status === 'superseded'
    ) {
      metadata.confirmedHash = entry.hash;
      metadata.decisionRef = input.decisionRef;
    }
    entry.history ??= [];
    entry.history.push({ hash: entry.hash, metadata: entry.metadata });
    entry.metadata = metadata;
    catalog.revision++;
    catalog.annotations.push({
      at: new Date().toISOString(),
      operator: userInfo().username,
      reason: input.reason,
      documentId: input.documentId,
      decisionRef: input.decisionRef,
    });
    await store.writeCatalog(catalog);
    return catalog;
  });
}

export async function readSourceAttachment(
  workspace: ResolvedWorkspace,
  sourceId: string,
  relativePath: string,
): Promise<Buffer> {
  const source = workspace.definition.sources.find((s) => s.id === sourceId);
  if (!source) {
    fail('missing_source', 'Unknown source', 3);
  }
  if (
    !/\.(?:png|jpe?g|gif|webp|avif)$/i.test(relativePath) ||
    excluded(relativePath, source.exclude) ||
    relativePath.split('/').some((p) => p.startsWith('.'))
  ) {
    fail('unsupported_attachment', 'Attachment is not permitted', 3);
  }
  const root = workspace.bindings[source.binding];
  if (!root) {
    fail('missing_binding', 'Source binding unavailable', 3);
  }
  if (workspacePrivate(workspace, root, relativePath)) {
    fail('private_path', 'Runtime and configuration files are private', 3);
  }
  return readFile(await contained(root, relativePath));
}
export async function readIndexedAsset(
  workspace: ResolvedWorkspace,
  store: KnowledgeStore,
  revision: string,
  sourceId: string,
  relativePath: string,
): Promise<{ bytes: Buffer; hash: string; mimeType: string }> {
  store.assertSpace(workspace);
  const index = await store.readIndex(revision);
  const asset = index?.assets?.find(
    (a) => a.sourceId === sourceId && a.relativePath === relativePath,
  );
  if (!index || index.workspaceId !== workspace.definition.id || !asset) {
    fail(
      'missing_indexed_asset',
      'Asset is not available in this source revision',
      3,
    );
  }
  return {
    bytes: await readObject(store, asset.objectPath, asset.hash),
    hash: asset.hash,
    mimeType: asset.mimeType,
  };
}
