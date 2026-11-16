import assert from 'node:assert/strict';
import { mkdtemp, mkdir, readFile, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { createRequire } from 'node:module';
import { join } from 'node:path';
import test from 'node:test';
import {
  ensureSelfBindings,
  migrateSelfBindings,
} from '../../../scripts/setup.mjs';

test('fresh checkout setup creates only private local bindings', async (t) => {
  const root = await mkdtemp(join(tmpdir(), 'ai2workspace-setup-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  assert.equal(await ensureSelfBindings(root), true);
  const config = JSON.parse(
    await readFile(join(root, 'spaces/self/workspace.local.yaml'), 'utf8'),
  );
  assert.equal(config.bindings['service-root'], root);
  assert.equal(config.bindings['knowledge-root'], join(root, 'src/content'));
  assert.equal(
    config.bindings['templates-root'],
    join(root, 'harness/templates'),
  );
  assert.equal(config.dataDir, '.local');
});
test('setup preserves every byte of an existing custom binding file', async (t) => {
  const root = await mkdtemp(join(tmpdir(), 'ai2workspace-setup-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  await mkdir(join(root, 'spaces/self'), { recursive: true });
  const target = join(root, 'spaces/self/workspace.local.yaml');
  const text = '# User-owned custom configuration\nbindings: {}\n';
  await writeFile(target, text);
  assert.equal(await ensureSelfBindings(root), false);
  assert.equal(await readFile(target, 'utf8'), text);
});

test('explicit layout migration updates only old defaults and retains comments, custom paths and task assets', async (t) => {
  const root = await mkdtemp(join(tmpdir(), 'ai2workspace-migrate-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  for (const directory of [
    'docs',
    'templates',
    'src/content',
    'harness/templates',
    'custom',
    'spaces/self/.local/tasks',
  ]) {
    await mkdir(join(root, directory), { recursive: true });
  }
  const directory = join(root, 'spaces/self');
  await writeFile(
    join(directory, 'workspace.yaml'),
    JSON.stringify({
      schemaVersion: 1,
      id: 'self',
      name: '项目',
      profile: 'development',
      sources: [
        {
          id: 'docs',
          binding: 'knowledge-root',
          kind: 'knowledge',
          include: ['**/*.md'],
          writePolicy: 'read-only',
        },
      ],
      projects: [],
      checks: [],
    }),
  );
  const target = join(directory, 'workspace.local.yaml');
  await writeFile(
    target,
    '# Keep this comment\nschemaVersion: 1\nbindings:\n  knowledge-root: ' +
      JSON.stringify(join(root, 'docs')) +
      '\n  templates-root: ' +
      JSON.stringify(join(root, 'templates')) +
      '\n  custom: ' +
      JSON.stringify(join(root, 'custom')) +
      '\nautoScan: false\ndataDir: .local\n',
  );
  const asset = join(directory, '.local/tasks/previous.json');
  await writeFile(asset, 'existing sealed task');
  assert.equal(await migrateSelfBindings(root), true);
  const updated = await readFile(target, 'utf8');
  assert.ok(updated.includes('# Keep this comment'));
  const require = createRequire(
    new URL('../../../packages/core/package.json', import.meta.url),
  );
  const config = require('yaml').parse(updated);
  assert.equal(config.bindings['knowledge-root'], join(root, 'src/content'));
  assert.equal(
    config.bindings['templates-root'],
    join(root, 'harness/templates'),
  );
  assert.equal(config.bindings.custom, join(root, 'custom'));
  assert.equal(config.dataDir, '.local');
  assert.equal(await readFile(asset, 'utf8'), 'existing sealed task');
  assert.equal(await migrateSelfBindings(root), false);
  assert.equal(await readFile(target, 'utf8'), updated);
});
