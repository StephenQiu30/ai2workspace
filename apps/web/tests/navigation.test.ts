import assert from 'node:assert/strict';
import test from 'node:test';
import * as adapter from '../lib/knowledge-source.ts';
import type { ScanResult } from '@ai2workspace/core/knowledge';
import type { SourceDefinition } from '@ai2workspace/core/workspace';

test('the reader presents pinned category titles and order with instructions separated from content', () => {
  const build = (
    adapter as unknown as {
      buildKnowledgeSource?: (
        scan: ScanResult,
        sources: SourceDefinition[],
        baseUrl: string,
        legacy: boolean,
      ) => { getPageTree: () => { children: unknown[] } };
    }
  ).buildKnowledgeSource;
  assert.equal(typeof build, 'function');
  const scan: ScanResult = {
    workspaceId: 'one',
    sourceRevision: 'old',
    issues: [],
    documents: [
      {
        workspaceId: 'one',
        sourceId: 'docs',
        documentId: 'intro',
        relativePath: 'product/intro.md',
        title: '产品介绍',
        body: '# 产品介绍',
        hash: 'a',
        metadata: { status: 'unclassified' },
        links: [],
      },
      {
        workspaceId: 'one',
        sourceId: 'docs',
        documentId: 'design',
        relativePath: 'architecture/design.md',
        title: '架构介绍',
        body: '# 架构介绍',
        hash: 'b',
        metadata: { status: 'unclassified' },
        links: [],
      },
      {
        workspaceId: 'one',
        sourceId: 'rules',
        documentId: 'agents',
        relativePath: 'AGENTS.md',
        title: '协作规则',
        body: '# 协作规则',
        hash: 'c',
        metadata: { status: 'unclassified' },
        links: [],
      },
    ],
    navigation: [
      {
        sourceId: 'docs',
        relativePath: 'meta.json',
        pages: ['product', 'architecture'],
      },
      {
        sourceId: 'docs',
        relativePath: 'product/meta.json',
        title: '产品',
        defaultOpen: true,
      },
      {
        sourceId: 'docs',
        relativePath: 'architecture/meta.json',
        title: '架构',
      },
    ],
  };
  const sources: SourceDefinition[] = [
    {
      id: 'docs',
      binding: 'docs',
      kind: 'knowledge',
      include: ['**/*.md'],
      exclude: [],
      writePolicy: 'read-only',
    },
    {
      id: 'rules',
      binding: 'rules',
      kind: 'instruction',
      include: ['AGENTS.md'],
      exclude: [],
      writePolicy: 'read-only',
    },
  ];
  const tree = build!(scan, sources, '/spaces/one/read', false).getPageTree();
  const children = tree.children as {
    name: string;
    children?: { url?: string }[];
    defaultOpen?: boolean;
  }[];
  assert.deepEqual(
    children.map((x) => x.name),
    ['产品', '架构', '适用规范'],
  );
  assert.equal(children[0]?.defaultOpen, true);
  assert.equal(
    children[0]?.children?.[0]?.url,
    '/spaces/one/read/docs/intro?revision=old',
  );
  assert.equal(
    children[2]?.children?.[0]?.url,
    '/spaces/one/read/rules/agents?revision=old',
  );
  const inferred = build!(
    { ...scan, navigation: undefined },
    sources,
    '/spaces/one/read',
    false,
  ).getPageTree();
  assert.ok(
    inferred.children.length >= 3,
    'plain Markdown still has inferred directory navigation',
  );
});

test('a content directory named like an instruction source keeps both identities and their separate navigation', () => {
  const build = (
    adapter as unknown as {
      buildKnowledgeSource: (
        scan: ScanResult,
        sources: SourceDefinition[],
        baseUrl: string,
      ) => {
        getPage: (slugs: string[]) => { data: { title: string } } | undefined;
        getPageTree: () => { children: { name: string }[] };
      };
    }
  ).buildKnowledgeSource;
  const doc = {
    workspaceId: 'one',
    relativePath: 'rules/AGENTS.md',
    title: '用户的普通资料',
    body: '# 用户的普通资料',
    hash: 'a',
    metadata: { status: 'unclassified' as const },
    links: [],
  };
  const scan: ScanResult = {
    workspaceId: 'one',
    sourceRevision: 'rev',
    issues: [],
    documents: [
      { ...doc, sourceId: 'notes', documentId: 'ordinary' },
      {
        ...doc,
        relativePath: 'AGENTS.md',
        sourceId: 'rules',
        documentId: 'instruction',
        title: '实际工程规范',
      },
    ],
    navigation: [
      {
        sourceId: 'notes',
        relativePath: 'rules/meta.json',
        title: '普通资料分类',
      },
    ],
  };
  const sources: SourceDefinition[] = [
    {
      id: 'notes',
      binding: 'a',
      kind: 'knowledge',
      include: ['**/*.md'],
      exclude: [],
      writePolicy: 'read-only',
    },
    {
      id: 'rules',
      binding: 'b',
      kind: 'instruction',
      include: ['AGENTS.md'],
      exclude: [],
      writePolicy: 'read-only',
    },
  ];
  const reader = build(scan, sources, '/spaces/one/read');
  assert.equal(
    reader.getPage(['notes', 'ordinary'])?.data.title,
    '用户的普通资料',
  );
  assert.equal(
    reader.getPage(['rules', 'instruction'])?.data.title,
    '实际工程规范',
  );
  assert.deepEqual(
    reader.getPageTree().children.map((n) => n.name),
    ['普通资料分类', '适用规范'],
  );
});

test('a pinned source classification survives adding empty sources or removing current instruction declarations', () => {
  const scan: ScanResult & {
    sources?: { id: string; kind: 'knowledge' | 'instruction' }[];
  } = {
    workspaceId: 'one',
    sourceRevision: 'pinned',
    issues: [],
    sources: [
      { id: 'docs', kind: 'knowledge' },
      { id: 'rules', kind: 'instruction' },
    ],
    documents: [
      {
        workspaceId: 'one',
        sourceId: 'docs',
        documentId: 'intro',
        relativePath: 'product/intro.md',
        title: '产品介绍',
        body: '# Intro',
        hash: 'a',
        metadata: { status: 'unclassified' },
        links: [],
      },
      {
        workspaceId: 'one',
        sourceId: 'rules',
        documentId: 'agents',
        relativePath: 'AGENTS.md',
        title: '规则',
        body: '# Rules',
        hash: 'b',
        metadata: { status: 'unclassified' },
        links: [],
      },
    ],
    navigation: [
      { sourceId: 'docs', relativePath: 'product/meta.json', title: '产品' },
    ],
  };
  const current: SourceDefinition[] = [
    {
      id: 'docs',
      kind: 'knowledge',
      binding: 'docs',
      include: ['**/*.md'],
      exclude: [],
      writePolicy: 'read-only',
    },
    {
      id: 'empty',
      kind: 'knowledge',
      binding: 'empty',
      include: ['**/*.md'],
      exclude: [],
      writePolicy: 'read-only',
    },
  ];
  const reader = adapter.buildKnowledgeSource(
    scan,
    current,
    '/spaces/one/read',
  );
  assert.deepEqual(
    reader.getPageTree().children.map((x) => x.name),
    ['产品', '适用规范'],
  );
});
