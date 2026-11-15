import assert from 'node:assert/strict';
import test from 'node:test';
import {
  resolveDocumentLink,
  resolveAttachmentPath,
  documentUrl,
} from '../lib/source-links.ts';

const documents = [
  { sourceId: 'notes', documentId: 'intro', relativePath: 'guide/intro.md' },
  {
    sourceId: 'notes',
    documentId: 'decision',
    relativePath: 'decisions/final.md',
  },
  { sourceId: 'rules', documentId: 'root', relativePath: 'AGENTS.md' },
];
test('relative links resolve only in the originating source and keep the pinned revision', () => {
  assert.equal(
    resolveDocumentLink(
      '../decisions/final.md#choice',
      documents[0],
      documents,
      '/spaces/one/read',
      'abc',
    ),
    '/spaces/one/read/notes/decision?revision=abc#choice',
  );
  assert.equal(
    resolveDocumentLink(
      'https://example.com',
      documents[0],
      documents,
      '/read',
      'abc',
    ),
    'https://example.com',
  );
  assert.equal(
    resolveDocumentLink(
      '../../AGENTS.md',
      documents[0],
      documents,
      '/read',
      'abc',
    ),
    undefined,
  );
});
test('attachments reject traversal, active content, and absolute paths', () => {
  assert.equal(
    resolveAttachmentPath('../images/diagram.png', 'guide/intro.md'),
    'images/diagram.png',
  );
  for (const unsafe of [
    '../../secret.png',
    '/etc/passwd',
    '../x.svg',
    'javascript:alert(1)',
    '../x.html',
  ]) {
    assert.equal(
      resolveAttachmentPath(unsafe, 'guide/intro.md'),
      undefined,
      unsafe,
    );
  }
});

test('root rules resolve only when the destination is explicitly present in another bound source', () => {
  const roots = { notes: '/project/knowledge', rules: '/project' };
  assert.equal(
    resolveDocumentLink(
      '../../AGENTS.md',
      documents[0],
      documents,
      '/spaces/one/read',
      'abc',
      roots,
    ),
    '/spaces/one/read/rules/root?revision=abc',
  );
  assert.equal(
    resolveDocumentLink(
      '../../PRIVATE.md',
      documents[0],
      documents,
      '/spaces/one/read',
      'abc',
      roots,
    ),
    undefined,
  );
});

test('legacy self aliases are reserved for the original knowledge source', () => {
  assert.equal(
    documentUrl(
      { sourceId: 'knowledge', documentId: 'index', relativePath: 'index.md' },
      '/read',
      'abc',
    ),
    '/read?revision=abc',
  );
  assert.equal(
    documentUrl(
      { sourceId: 'rules', documentId: 'index', relativePath: 'AGENTS.md' },
      '/read',
      'abc',
    ),
    '/read/rules/index?revision=abc',
  );
});
