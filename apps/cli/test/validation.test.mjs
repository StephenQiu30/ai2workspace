import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtemp, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import test from 'node:test';

test('a declared but missing local binding is blocked instead of a ready validation', async (t) => {
  const dir = await mkdtemp(join(tmpdir(), 'ai2workspace-unbound-'));
  t.after(() => rm(dir, { recursive: true, force: true }));
  await writeFile(
    join(dir, 'workspace.yaml'),
    JSON.stringify({
      schemaVersion: 1,
      id: 'unbound',
      name: 'Unbound',
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
    join(dir, 'workspace.local.yaml'),
    JSON.stringify({ schemaVersion: 1, bindings: {}, autoScan: false }),
  );
  const result = spawnSync(
    process.execPath,
    [
      resolve('dist/index.js'),
      '--workspace',
      dir,
      'space',
      'validate',
      '--json',
    ],
    { encoding: 'utf8' },
  );
  const output = JSON.parse(result.stdout);
  assert.equal(result.status, 3);
  assert.equal(output.ok, false);
  assert.ok(output.issues.some((issue) => issue.code === 'missing_binding'));
});
