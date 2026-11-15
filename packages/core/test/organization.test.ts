import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, writeFile, rename, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import * as core from '../src/index.js';
import type { ScanResult, KnowledgeDocument } from '../src/contracts.js';

test('navigation changes create a new revision while historical navigation and document identity remain pinned', async (t) => {
  const root = await mkdtemp(path.join(tmpdir(), 'ai2-navigation-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const docs = path.join(root, 'docs');
  await mkdir(docs);
  await writeFile(path.join(docs, 'one.md'), '# One');
  await writeFile(
    path.join(docs, 'document-ids.json'),
    JSON.stringify([{ id: 'legacy', path: 'one.md' }]),
  );
  await writeFile(
    path.join(docs, 'meta.json'),
    JSON.stringify({ title: '原目录', pages: ['one'], defaultOpen: true }),
  );
  await writeFile(
    path.join(root, 'workspace.yaml'),
    JSON.stringify({
      schemaVersion: 1,
      id: 'one',
      name: 'one',
      profile: 'generic',
      sources: [
        {
          id: 'docs',
          binding: 'docs',
          kind: 'knowledge',
          include: ['**/*.md'],
          exclude: [],
          writePolicy: 'read-only',
        },
      ],
      projects: [],
      checks: [],
    }),
  );
  await writeFile(
    path.join(root, 'workspace.local.yaml'),
    JSON.stringify({ schemaVersion: 1, bindings: { docs } }),
  );
  const workspace = await core.resolveWorkspace(root);
  const store = await core.openSpaceStore(workspace);
  const before = await core.scanSources(workspace, store);
  await writeFile(
    path.join(docs, 'meta.json'),
    JSON.stringify({ title: '新目录', pages: ['one'], defaultOpen: false }),
  );
  const after = await core.scanSources(workspace, store);
  assert.notEqual(
    after.sourceRevision,
    before.sourceRevision,
    'navigation must participate in the knowledge revision',
  );
  type NavScan = ScanResult & {
    navigation?: { sourceId: string; relativePath: string; title?: string }[];
  };
  assert.equal(
    ((await store.readIndex(before.sourceRevision)) as NavScan).navigation?.[0]
      ?.title,
    '原目录',
  );
  assert.equal((after as NavScan).navigation?.[0]?.title, '新目录');
  await rename(path.join(docs, 'one.md'), path.join(docs, 'moved.md'));
  await writeFile(
    path.join(docs, 'document-ids.json'),
    JSON.stringify([{ id: 'legacy', path: 'moved.md' }]),
  );
  const moved = await core.scanSources(workspace, store);
  assert.equal(moved.documents[0]?.documentId, 'legacy');
  assert.equal(
    (await store.readIndex(before.sourceRevision))?.documents[0]?.relativePath,
    'one.md',
  );
  workspace.definition.sources[0]!.kind = 'reference';
  const reclassified = await core.scanSources(workspace, store);
  assert.notEqual(
    reclassified.sourceRevision,
    moved.sourceRevision,
    'source presentation classification must be revisioned even when document bytes are unchanged',
  );
});

test('context selection works without tasks or projects and enforces dependency confirmation', () => {
  const select = (
    core as unknown as {
      selectContext?: (
        scan: ScanResult,
        request: {
          inputs: {
            workspaceId: string;
            sourceId: string;
            documentId: string;
            purpose: string;
            required: boolean;
            requireConfirmed?: boolean;
          }[];
        },
      ) => {
        documents: { doc: KnowledgeDocument }[];
        issues: { code: string }[];
        missingOptional: unknown[];
      };
    }
  ).selectContext;
  assert.equal(
    typeof select,
    'function',
    'knowledge context must be usable without a task lifecycle',
  );
  const doc: KnowledgeDocument = {
    workspaceId: 'one',
    sourceId: 'notes',
    documentId: 'guide',
    relativePath: 'guide.md',
    title: 'Guide',
    body: '# Guide',
    hash: 'body',
    metadata: {
      status: 'draft',
      dependencies: [
        { workspaceId: 'one', sourceId: 'notes', documentId: 'basis' },
      ],
    },
    links: [],
  };
  const basis: KnowledgeDocument = {
    ...doc,
    documentId: 'basis',
    relativePath: 'basis.md',
    metadata: { status: 'unclassified' },
  };
  const scan: ScanResult = {
    workspaceId: 'one',
    sourceRevision: 'rev',
    documents: [doc, basis],
    issues: [],
  };
  const input = {
    workspaceId: 'one',
    sourceId: 'notes',
    documentId: 'guide',
    purpose: 'reading',
    required: true,
  };
  const chosen = select!(scan, { inputs: [input] });
  assert.deepEqual(
    chosen.documents.map((x) => x.doc.documentId),
    ['guide', 'basis'],
  );
  assert.deepEqual(chosen.issues, []);
  const strict = select!(scan, {
    inputs: [input, { ...input, requireConfirmed: true }],
  });
  assert.ok(strict.issues.some((x) => x.code === 'confirmation_required'));
  assert.equal(doc.metadata.status, 'draft');
});
