import assert from 'node:assert/strict';
import { mkdtemp, mkdir, readFile, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { ensureSelfBindings } from '../../../tools/setup.mjs';

test('fresh checkout setup creates only private local bindings', async (t) => {
  const root = await mkdtemp(join(tmpdir(), 'ai2workspace-setup-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  assert.equal(await ensureSelfBindings(root), true);
  const config = JSON.parse(
    await readFile(join(root, 'spaces/self/workspace.local.yaml'), 'utf8'),
  );
  assert.equal(config.bindings['service-root'], root);
  assert.equal(config.bindings['knowledge-root'], join(root, 'docs'));
  assert.equal(config.bindings['templates-root'], join(root, 'templates'));
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
