import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtemp, writeFile, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import * as core from '@ai2workspace/core';

const digest = (text) => createHash('sha256').update(text).digest('hex');
const valid = {
  schemaVersion: 1,
  id: 'config-test',
  name: 'Configuration',
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
};
async function fixture(t, input = valid) {
  const dir = await mkdtemp(join(tmpdir(), 'ai2workspace-config-'));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const text = JSON.stringify(input);
  await writeFile(join(dir, 'workspace.yaml'), text);
  await writeFile(
    join(dir, 'workspace.local.yaml'),
    JSON.stringify({
      schemaVersion: 1,
      bindings: { docs: dir },
      autoScan: false,
    }),
  );
  return { dir, text };
}
test('a semantically broken declaration is repairable without resolving the broken space', async (t) => {
  assert.equal(typeof core.updateWorkspaceDefinition, 'function');
  const { dir, text } = await fixture(t, {
    ...valid,
    sources: [valid.sources[0], valid.sources[0]],
  });
  const result = await core.updateWorkspaceDefinition(
    dir,
    JSON.stringify(valid),
    digest(text),
  );
  assert.equal(result.id, 'config-test');
  assert.deepEqual(
    JSON.parse(await readFile(join(dir, 'workspace.yaml'), 'utf8')).sources,
    valid.sources,
  );
});
test('invalid new cross references never replace a valid declaration', async (t) => {
  const { dir, text } = await fixture(t);
  await assert.rejects(() =>
    core.updateWorkspaceDefinition(
      dir,
      JSON.stringify({
        ...valid,
        sources: [{ ...valid.sources[0], projectId: 'unknown' }],
      }),
      digest(text),
    ),
  );
  assert.equal(await readFile(join(dir, 'workspace.yaml'), 'utf8'), text);
});
test('a stale editor revision cannot overwrite a concurrent update', async (t) => {
  const { dir } = await fixture(t);
  await assert.rejects(
    () =>
      core.updateWorkspaceDefinition(
        dir,
        JSON.stringify(valid),
        '0'.repeat(64),
      ),
    (error) => error.exitCode === 5,
  );
});
test('configuration edits obey the running space execution lock', async (t) => {
  const { dir, text } = await fixture(t);
  const store = await core.openSpaceStore(await core.resolveWorkspace(dir));
  await store.withLock(async () => {
    await assert.rejects(
      () =>
        core.updateWorkspaceDefinition(
          dir,
          JSON.stringify({ ...valid, name: 'New' }),
          digest(text),
        ),
      (error) => error.exitCode === 5,
    );
  });
  assert.equal(await readFile(join(dir, 'workspace.yaml'), 'utf8'), text);
});

test('stale configuration remains a revision conflict while a background read holds the lock', async (t) => {
  const { dir, text } = await fixture(t);
  const store = await core.openSpaceStore(await core.resolveWorkspace(dir));
  await store.withLock(async () => {
    await assert.rejects(
      core.updateWorkspaceDefinition(
        dir,
        JSON.stringify(valid),
        '0'.repeat(64),
      ),
      (error) =>
        error.issues.some((issue) => issue.code === 'revision_conflict'),
    );
  });
  assert.equal(await readFile(join(dir, 'workspace.yaml'), 'utf8'), text);
});
