import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtemp, mkdir, writeFile, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve, join } from 'node:path';
import test from 'node:test';

const binary = resolve(process.env.AI2WORKSPACE_CLI_ENTRY ?? 'dist/index.js');
function call(workspace, ...args) {
  const result = spawnSync(
    process.execPath,
    [binary, '--workspace', workspace, ...args, '--json'],
    { encoding: 'utf8' },
  );
  let output;
  try {
    output = JSON.parse(result.stdout);
  } catch {
    assert.fail(
      `CLI must emit one JSON envelope; stderr=${result.stderr}, stdout=${result.stdout}`,
    );
  }
  return { ...result, output };
}
async function fixture(t) {
  const base = await mkdtemp(join(tmpdir(), 'ai2workspace-cli-'));
  t.after(() => rm(base, { recursive: true, force: true }));
  const workspace = join(base, 'space');
  const project = join(base, 'project');
  const docs = join(base, 'docs');
  await Promise.all(
    [workspace, project, docs].map((path) => mkdir(path, { recursive: true })),
  );
  await writeFile(join(project, 'existing.txt'), 'user work\n');
  await writeFile(join(docs, 'guide.md'), '# Guide\n\nA stable source.\n');
  const definition = {
    schemaVersion: 1,
    id: 'fixture',
    name: 'Fixture',
    profile: 'development',
    sources: [
      {
        id: 'docs',
        binding: 'docs',
        kind: 'knowledge',
        include: ['**/*.md'],
        exclude: [],
        writePolicy: 'task-scoped',
      },
    ],
    projects: [{ id: 'code', binding: 'code', inputExclude: [] }],
    checks: [
      {
        id: 'success',
        projectId: 'code',
        cwd: '.',
        argv: [process.execPath, '-e', 'process.stdout.write("checked")'],
        timeoutMs: 5000,
        required: true,
        tools: [],
        environment: { publicNames: [], privateRefs: [] },
        artifacts: [],
      },
      {
        id: 'failure',
        projectId: 'code',
        cwd: '.',
        argv: [process.execPath, '-e', 'process.exit(7)'],
        timeoutMs: 5000,
        required: true,
        tools: [],
        environment: { publicNames: [], privateRefs: [] },
        artifacts: [],
      },
      {
        id: 'repairable',
        projectId: 'code',
        cwd: '.',
        argv: [
          process.execPath,
          '-e',
          'process.exit(require("node:fs").existsSync("fixed.txt") ? 0 : 7)',
        ],
        timeoutMs: 5000,
        required: true,
        tools: [],
        environment: { publicNames: [], privateRefs: [] },
        artifacts: [],
      },
    ],
  };
  await writeFile(
    join(workspace, 'workspace.yaml'),
    JSON.stringify(definition),
  );
  await writeFile(
    join(workspace, 'workspace.local.yaml'),
    JSON.stringify({
      schemaVersion: 1,
      bindings: { docs, code: project },
      autoScan: false,
    }),
  );
  return { base, workspace, project, docs, definition };
}
async function prepare(t, checkId = 'success') {
  const f = await fixture(t);
  assert.equal(call(f.workspace, 'space', 'validate').status, 0);
  const scanned = call(f.workspace, 'source', 'scan');
  assert.equal(scanned.status, 0);
  const document = scanned.output.data.documents[0];
  const contract = {
    schemaVersion: 1,
    id: 'task-one',
    workspaceId: 'fixture',
    kind: 'implementation',
    goal: 'Verify the declared project',
    scope: { outputs: [] },
    projectIds: ['code'],
    inputs: [
      {
        workspaceId: 'fixture',
        sourceId: 'docs',
        documentId: document.documentId,
        purpose: 'baseline',
        required: true,
      },
    ],
    acceptance: [
      { id: 'ac-one', checkIds: [checkId], kind: 'automatic', required: true },
    ],
    checkIds: [checkId],
    delivery: { artifacts: [], requireHumanAcceptance: false },
  };
  const file = join(f.base, 'contract.json');
  await writeFile(file, JSON.stringify(contract));
  assert.equal(
    call(f.workspace, 'task', 'create', '--contract', file).status,
    0,
  );
  const prepared = call(f.workspace, 'task', 'prepare', '--task', 'task-one');
  assert.equal(prepared.status, 0, JSON.stringify(prepared.output));
  return { ...f, prepared: prepared.output.data, contract };
}

test('help exposes the Codex workflow without requiring a space', () => {
  const result = spawnSync(process.execPath, [binary, '--help'], {
    encoding: 'utf8',
  });
  assert.equal(result.status, 0);
  assert.match(result.stdout, /task prepare/);
  assert.match(result.stdout, /task handoff/);
});
test('unknown schema is a structured input error', async (t) => {
  const f = await fixture(t);
  await writeFile(
    join(f.workspace, 'workspace.yaml'),
    JSON.stringify({ ...f.definition, schemaVersion: 2 }),
  );
  const result = call(f.workspace, 'space', 'validate');
  assert.equal(result.status, 2);
  assert.equal(result.output.ok, false);
  assert.ok(result.output.issues.length);
});
test('unknown options cannot override a declared command', async (t) => {
  const f = await fixture(t);
  const result = call(
    f.workspace,
    'check',
    'run',
    '--task',
    'task-one',
    '--check',
    'success',
    '--argv',
    'echo forged',
  );
  assert.equal(result.status, 2);
  assert.equal(result.output.ok, false);
});
test('independent CLI processes prepare, check, hand off and resume without changing user files', async (t) => {
  const f = await prepare(t);
  const exported = call(f.workspace, 'context', 'export', '--task', 'task-one');
  assert.equal(exported.status, 0);
  assert.match(exported.output.data.documents[0].body, /A stable source/);
  const run = call(
    f.workspace,
    'check',
    'run',
    '--task',
    'task-one',
    '--check',
    'success',
  );
  assert.equal(run.status, 0, JSON.stringify(run.output));
  assert.equal(run.output.data.result, 'passed');
  assert.equal(
    call(f.workspace, 'task', 'verify', '--task', 'task-one').status,
    0,
  );
  const notes = join(f.base, 'notes.json');
  await writeFile(
    notes,
    JSON.stringify({
      schemaVersion: 1,
      taskId: 'task-one',
      contractRevision: 1,
      snapshotId: f.prepared.snapshotId,
      completedClaims: [
        {
          text: 'Declared check passed',
          evidenceRunIds: [run.output.data.runId],
        },
      ],
      openIssues: [],
      nextSteps: ['Review delivery'],
      externalActions: [],
    }),
  );
  const handoff = call(
    f.workspace,
    'task',
    'handoff',
    '--task',
    'task-one',
    '--notes',
    notes,
  );
  assert.equal(handoff.status, 0, JSON.stringify(handoff.output));
  const resume = call(f.workspace, 'task', 'resume', '--task', 'task-one');
  assert.equal(resume.status, 0, JSON.stringify(resume.output));
  assert.ok(resume.output.data.reusableRunIds.includes(run.output.data.runId));
  assert.equal(
    call(f.workspace, 'task', 'complete', '--task', 'task-one').status,
    0,
  );
  assert.equal(
    await readFile(join(f.project, 'existing.txt'), 'utf8'),
    'user work\n',
  );
});
test('a failing process exits 1 and never verifies as passed', async (t) => {
  const f = await prepare(t, 'failure');
  const result = call(
    f.workspace,
    'check',
    'run',
    '--task',
    'task-one',
    '--check',
    'failure',
  );
  assert.equal(result.status, 1);
  assert.equal(result.output.data.result, 'failed');
  assert.notEqual(
    call(f.workspace, 'task', 'verify', '--task', 'task-one').status,
    0,
  );
});
test('candidate edits make previously captured evidence stale', async (t) => {
  const f = await prepare(t);
  assert.equal(
    call(
      f.workspace,
      'check',
      'run',
      '--task',
      'task-one',
      '--check',
      'success',
    ).status,
    0,
  );
  await writeFile(join(f.project, 'existing.txt'), 'user changed their work\n');
  assert.equal(
    call(f.workspace, 'task', 'verify', '--task', 'task-one').status,
    4,
  );
  assert.equal(
    await readFile(join(f.project, 'existing.txt'), 'utf8'),
    'user changed their work\n',
  );
});
test('a fresh passing run supersedes historical stale failure after a real fix', async (t) => {
  const f = await prepare(t, 'repairable');
  assert.equal(
    call(
      f.workspace,
      'check',
      'run',
      '--task',
      'task-one',
      '--check',
      'repairable',
    ).status,
    1,
  );
  await writeFile(join(f.project, 'fixed.txt'), 'actual fix\n');
  assert.equal(
    call(
      f.workspace,
      'check',
      'run',
      '--task',
      'task-one',
      '--check',
      'repairable',
    ).status,
    0,
  );
  const result = call(f.workspace, 'task', 'verify', '--task', 'task-one');
  assert.equal(result.output.data.condition, 'clear');
  assert.equal(result.status, 0);
});
test('changed basis is a stale preparation result rather than an environment block', async (t) => {
  const f = await prepare(t);
  await writeFile(join(f.docs, 'guide.md'), '# Guide\n\nChanged basis.\n');
  const result = call(f.workspace, 'task', 'prepare', '--task', 'task-one');
  assert.equal(result.status, 4);
  assert.equal(result.output.ok, false);
});
