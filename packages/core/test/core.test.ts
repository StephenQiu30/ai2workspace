import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, writeFile, readFile, symlink } from 'node:fs/promises';
import { tmpdir, hostname } from 'node:os';
import path from 'node:path';
import * as core from '../src/index.js';
import type { TaskInput } from '../src/contracts.js';
async function fixture(
  id = 'one',
  script = 'console.log(process.cwd())',
  extraCheck: object = {},
) {
  const root = await mkdtemp(path.join(tmpdir(), 'ai2-core-'));
  const ws = path.join(root, 'ws');
  const repo = path.join(root, 'repo');
  await mkdir(ws);
  await mkdir(repo);
  await writeFile(path.join(repo, 'guide.md'), '# Guide\nOriginal\n');
  await writeFile(path.join(repo, 'AGENTS.md'), '# Rules\nStay scoped\n');
  await writeFile(path.join(repo, 'app.txt'), 'initial user changes');
  const definition = {
    schemaVersion: 1,
    id,
    name: id,
    profile: 'generic',
    sources: [
      {
        id: 'docs',
        binding: 'repo',
        kind: 'knowledge',
        include: ['*.md'],
        writePolicy: 'task-scoped',
      },
      {
        id: 'rules',
        binding: 'repo',
        kind: 'instruction',
        include: ['AGENTS.md'],
        projectId: 'app',
        writePolicy: 'read-only',
      },
    ],
    projects: [{ id: 'app', binding: 'repo', inputExclude: [] }],
    checks: [
      {
        id: 'test',
        projectId: 'app',
        cwd: '.',
        argv: [process.execPath, '-e', script],
        required: true,
        tools: [{ id: 'node', argv: [process.execPath, '--version'] }],
        ...extraCheck,
      },
    ],
  };
  await writeFile(path.join(ws, 'workspace.yaml'), JSON.stringify(definition));
  await writeFile(
    path.join(ws, 'workspace.local.yaml'),
    JSON.stringify({ schemaVersion: 1, bindings: { repo } }),
  );
  const workspace = await core.resolveWorkspace(ws);
  const store = await core.openSpaceStore(workspace);
  const scan = await core.scanSources(workspace, store);
  const doc = scan.documents.find(
    (d) => d.sourceId === 'docs' && d.relativePath === 'guide.md',
  )!;
  const input: TaskInput = {
    schemaVersion: 1,
    id: 'task',
    workspaceId: id,
    kind: 'implementation',
    goal: 'a real task',
    scope: { outputs: [] },
    projectIds: ['app'],
    inputs: [
      {
        workspaceId: id,
        sourceId: 'docs',
        documentId: doc.documentId,
        purpose: 'basis',
        required: true,
      },
    ],
    acceptance: [{ id: 'ac', checkIds: ['test'] }],
    checkIds: ['test'],
    delivery: { artifacts: [], requireHumanAcceptance: false },
  };
  return { root, ws, repo, workspace, store, scan, doc, input, definition };
}
test('rejects unknown schema instead of guessing', async () => {
  const d = await mkdtemp(path.join(tmpdir(), 'ai2-invalid-'));
  await writeFile(path.join(d, 'workspace.yaml'), 'schemaVersion: 2');
  await assert.rejects(
    core.resolveWorkspace(d),
    (e: unknown) => e instanceof core.CoreError && e.exitCode === 2,
  );
});
test('stable catalog IDs preserve explicit imported IDs and never execute Markdown', async () => {
  const f = await fixture();
  await writeFile(
    path.join(f.repo, 'document-ids.json'),
    JSON.stringify([{ id: 'legacy', path: 'legacy.md' }]),
  );
  await writeFile(
    path.join(f.repo, 'legacy.md'),
    '# Hello\n```sh\ntouch hacked\n```',
  );
  const a = await core.scanSources(f.workspace, f.store);
  const b = await core.scanSources(f.workspace, f.store);
  assert.equal(
    a.documents.find((d) => d.relativePath === 'legacy.md')?.documentId,
    'legacy',
  );
  assert.equal(a.documents[0]?.documentId, b.documents[0]?.documentId);
  await assert.rejects(readFile(path.join(f.repo, 'hacked')));
});
test('symlinks and private files are never scanned', async () => {
  const f = await fixture();
  await symlink('/etc/passwd', path.join(f.repo, 'secret.md'));
  await writeFile(path.join(f.repo, '.env.md'), 'private');
  const s = await core.scanSources(f.workspace, f.store);
  assert(
    !s.documents.some(
      (d) =>
        d.relativePath.includes('secret') || d.relativePath.includes('.env'),
    ),
  );
  assert(s.issues.some((i) => i.code === 'symlink_excluded'));
});
test('annotation conflict cannot overwrite confirmed hash', async () => {
  const f = await fixture();
  const cat = await f.store.readCatalog();
  await core.annotateCatalog(f.workspace, f.store, {
    schemaVersion: 1,
    workspaceId: f.workspace.definition.id,
    sourceId: f.doc.sourceId,
    documentId: f.doc.documentId,
    expectedRevision: cat.revision,
    fields: { status: 'confirmed' },
    reason: 'accepted',
    decisionRef: 'user:1',
  });
  await assert.rejects(
    core.annotateCatalog(f.workspace, f.store, {
      schemaVersion: 1,
      workspaceId: f.workspace.definition.id,
      sourceId: f.doc.sourceId,
      documentId: f.doc.documentId,
      expectedRevision: cat.revision,
      fields: { status: 'draft' },
      reason: 'old',
    }),
    (e: unknown) => e instanceof core.CoreError && e.exitCode === 5,
  );
  await writeFile(path.join(f.repo, 'guide.md'), '# Changed');
  const s = await core.scanSources(f.workspace, f.store);
  assert.equal(
    s.documents.find((d) => d.documentId === f.doc.documentId)?.metadata.status,
    'draft',
  );
});
test('snapshot holds actual rules and old body, relevant edits require explicit rebaseline', async () => {
  const f = await fixture();
  await core.createTask(f.workspace, f.store, f.input);
  const p = await core.prepareTask(f.workspace, f.store, 'task');
  assert.equal(p.ready, true);
  const ctx = await core.exportContext(f.workspace, f.store, 'task');
  assert(ctx.documents.some((d) => d.kind === 'instruction'));
  await writeFile(path.join(f.repo, 'guide.md'), '# Changed');
  assert(
    (await core.exportContext(f.workspace, f.store, 'task')).documents.some(
      (d) => d.body.includes('Original'),
    ),
  );
  assert.equal(
    (await core.prepareTask(f.workspace, f.store, 'task')).ready,
    false,
  );
  const origin = (await f.store.readTask('task')).originId;
  assert.equal(
    (
      await core.prepareTask(f.workspace, f.store, 'task', {
        rebaseline: true,
        reason: 'reviewed',
      })
    ).ready,
    true,
  );
  assert.equal((await f.store.readTask('task')).originId, origin);
  assert.equal(
    await readFile(path.join(f.repo, 'app.txt'), 'utf8'),
    'initial user changes',
  );
});
test('unrelated source edit does not stale selected basis', async () => {
  const f = await fixture();
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  await writeFile(path.join(f.repo, 'other.md'), '# Other');
  assert.equal(
    (await core.prepareTask(f.workspace, f.store, 'task')).ready,
    true,
  );
});
test('missing required document blocks ready', async () => {
  const f = await fixture();
  f.input.inputs[0]!.documentId = 'absent';
  await core.createTask(f.workspace, f.store, f.input);
  assert.equal(
    (await core.prepareTask(f.workspace, f.store, 'task')).ready,
    false,
  );
});
test('declared document output does not self-stale but remains candidate input', async () => {
  const f = await fixture();
  f.input.scope.outputs = [{ sourceId: 'docs', documentId: f.doc.documentId }];
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  await writeFile(path.join(f.repo, 'guide.md'), '# produced');
  assert.equal(
    (await core.prepareTask(f.workspace, f.store, 'task')).ready,
    true,
  );
});
test('real process pass binds candidate and current handoff gates complete', async () => {
  const f = await fixture();
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  const r = await core.runCheck(f.workspace, f.store, 'task', 'test');
  assert.equal(r.result, 'passed');
  assert.equal(r.evidenceSource, 'cli-captured');
  assert.equal(
    (await core.verifyTask(f.workspace, f.store, 'task')).phase,
    'verified',
  );
  await assert.rejects(core.completeTask(f.workspace, f.store, 'task'));
  const state = await f.store.readTask('task');
  const cp = await core.createHandoff(f.workspace, f.store, 'task', {
    schemaVersion: 1,
    taskId: 'task',
    contractRevision: 1,
    snapshotId: state.snapshotId!,
    completedClaims: [{ text: 'checked', evidenceRunIds: [r.runId] }],
    openIssues: [],
    nextSteps: [],
    externalActions: [],
  });
  assert(cp.summary.includes('checked'));
  assert.equal(
    (await core.resumeTask(f.workspace, f.store, 'task')).checkpointId,
    cp.id,
  );
  assert.equal(
    (await core.completeTask(f.workspace, f.store, 'task')).phase,
    'complete',
  );
});
test('changed candidate makes old pass unusable', async () => {
  const f = await fixture();
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  await core.runCheck(f.workspace, f.store, 'task', 'test');
  await writeFile(path.join(f.repo, 'app.txt'), 'new candidate');
  const v = await core.verifyTask(f.workspace, f.store, 'task');
  assert.notEqual(v.phase, 'verified');
  assert.equal(v.staleRunIds.length, 1);
});
test('old project report cannot satisfy current run artifact', async () => {
  const f = await fixture('one', "console.log('ok')", {
    artifacts: ['report.json'],
  });
  await writeFile(path.join(f.repo, 'report.json'), 'old pass');
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  const r = await core.runCheck(f.workspace, f.store, 'task', 'test');
  assert.equal(r.result, 'failed');
  assert(r.issues.some((i) => i.code === 'missing_artifact'));
});
test('new run-owned artifact is accepted', async () => {
  const f = await fixture(
    'one',
    "require('node:fs').writeFileSync(process.env.AI2WORKSPACE_RUN_DIR+'/artifacts/report.json','fresh')",
    { artifacts: ['report.json'] },
  );
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  assert.equal(
    (await core.runCheck(f.workspace, f.store, 'task', 'test')).result,
    'passed',
  );
});
test('missing executable is blocked and timeout cannot pass', async () => {
  for (const argv of [
    ['ai2workspace-no-such-executable'],
    [process.execPath, '-e', 'setInterval(()=>{},1000)'],
  ]) {
    const f = await fixture('one', '', { argv, timeoutMs: 100 });
    await core.createTask(f.workspace, f.store, f.input);
    await core.prepareTask(f.workspace, f.store, 'task');
    const r = await core.runCheck(f.workspace, f.store, 'task', 'test');
    assert.equal(r.result, argv.length === 1 ? 'blocked' : 'failed');
  }
});
test('input mutation during process fails even exit zero', async () => {
  const f = await fixture(
    'one',
    "require('node:fs').writeFileSync('app.txt','mutated')",
  );
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  const r = await core.runCheck(f.workspace, f.store, 'task', 'test');
  assert.equal(r.result, 'failed');
  assert(r.issues.some((i) => i.code === 'input_changed'));
});
test('private refs are blocked without hashing secret values', async () => {
  const f = await fixture('one', '', {
    environment: { privateRefs: ['TOKEN'], publicNames: [] },
  });
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  const r = await core.runCheck(f.workspace, f.store, 'task', 'test');
  assert.equal(r.result, 'blocked');
  assert(r.issues.some((i) => i.code === 'unsupported_environment'));
});
test('public environment drift invalidates evidence', async () => {
  const f = await fixture('one', '', {
    environment: { privateRefs: [], publicNames: ['AI2_TEST_PUBLIC'] },
  });
  process.env.AI2_TEST_PUBLIC = 'a';
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  await core.runCheck(f.workspace, f.store, 'task', 'test');
  process.env.AI2_TEST_PUBLIC = 'b';
  assert.notEqual(
    (await core.verifyTask(f.workspace, f.store, 'task')).phase,
    'verified',
  );
  delete process.env.AI2_TEST_PUBLIC;
});
test('revision and foreign space boundaries reject overwrites', async () => {
  const f = await fixture();
  await core.createTask(f.workspace, f.store, f.input);
  await assert.rejects(
    core.reviseTask(f.workspace, f.store, 'task', {
      contract: f.input,
      expectedRevision: 9,
      reason: 'wrong',
    }),
    (e: unknown) => e instanceof core.CoreError && e.exitCode === 5,
  );
  const b = await fixture('two');
  await assert.rejects(core.createTask(b.workspace, f.store, { ...b.input }));
  assert.equal((await f.store.readTask('task')).contractRevision, 1);
});
test('held and dead process locks require explicit recovery', async () => {
  const f = await fixture();
  await f.store.withLock(async () => {
    await assert.rejects(
      core.scanSources(f.workspace, f.store),
      (e: unknown) => e instanceof core.CoreError && e.exitCode === 5,
    );
  });
  await writeFile(
    path.join(f.store.root, 'space.lock'),
    JSON.stringify({ pid: 2147483647, host: hostname() }),
  );
  await assert.rejects(
    core.scanSources(f.workspace, f.store),
    (e: unknown) => e instanceof core.CoreError && e.exitCode === 5,
  );
  await f.store.recoverLock();
  assert((await core.scanSources(f.workspace, f.store)).documents.length > 0);
});
test('unavailable source keeps last complete index instead of publishing partial content', async () => {
  const f = await fixture();
  const revision = f.scan.sourceRevision;
  delete f.workspace.bindings.repo;
  f.workspace.issues = [
    {
      code: 'missing_binding',
      severity: 'error',
      message: 'Unavailable source',
    },
  ];
  await assert.rejects(
    core.scanSources(f.workspace, f.store),
    (e: unknown) => e instanceof core.CoreError && e.exitCode === 3,
  );
  assert.equal((await f.store.readIndex())?.sourceRevision, revision);
});
test('attachments remain actual immutable bytes after original changes', async () => {
  const f = await fixture();
  await writeFile(path.join(f.repo, 'image.png'), Buffer.from([1, 2, 3]));
  await writeFile(
    path.join(f.repo, 'guide.md'),
    '# Guide\n![Image](image.png)',
  );
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  const ctx = await core.exportContext(f.workspace, f.store, 'task');
  assert.equal(ctx.attachments.length, 1);
  await writeFile(path.join(f.repo, 'image.png'), Buffer.from([4]));
  assert.equal(
    (await core.prepareTask(f.workspace, f.store, 'task')).ready,
    false,
  );
  assert.deepEqual(
    await readFile(path.join(f.store.root, ctx.attachments[0]!.objectPath)),
    Buffer.from([1, 2, 3]),
  );
});
test('contract changes require explicit rebaseline and preserve old contract', async () => {
  const f = await fixture();
  await core.createTask(f.workspace, f.store, f.input);
  const p = await core.prepareTask(f.workspace, f.store, 'task');
  await core.runCheck(f.workspace, f.store, 'task', 'test');
  await core.reviseTask(f.workspace, f.store, 'task', {
    contract: { ...f.input, goal: 'revised goal' },
    expectedRevision: 1,
    reason: 'explicit',
  });
  assert.equal(
    (await core.prepareTask(f.workspace, f.store, 'task')).ready,
    false,
  );
  assert.equal((await f.store.readContract('task', 1)).goal, 'a real task');
  assert.equal(
    (
      await core.prepareTask(f.workspace, f.store, 'task', {
        rebaseline: true,
        reason: 'accepted',
      })
    ).ready,
    true,
  );
  assert.notEqual((await f.store.readTask('task')).snapshotId, p.snapshotId);
  assert.notEqual(
    (await core.verifyTask(f.workspace, f.store, 'task')).phase,
    'verified',
  );
});
test('handoff mismatched snapshot cannot replace current checkpoint', async () => {
  const f = await fixture();
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  await assert.rejects(
    core.createHandoff(f.workspace, f.store, 'task', {
      schemaVersion: 1,
      taskId: 'task',
      contractRevision: 9,
      snapshotId: 'snap_' + '0'.repeat(64),
      completedClaims: [],
      openIssues: [],
      nextSteps: [],
      externalActions: [],
    }),
    (e: unknown) => e instanceof core.CoreError && e.exitCode === 4,
  );
  assert.equal((await f.store.readTask('task')).checkpointId, undefined);
});
test('check cwd cannot traverse outside binding', async () => {
  const f = await fixture();
  f.definition.checks[0]!.cwd = '../outside';
  await writeFile(
    path.join(f.ws, 'workspace.yaml'),
    JSON.stringify(f.definition),
  );
  await assert.rejects(
    core.resolveWorkspace(f.ws),
    (e: unknown) => e instanceof core.CoreError && e.exitCode === 2,
  );
});
test('required process failure prevents verified even when query succeeds', async () => {
  const f = await fixture('one', 'process.exit(7)');
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  assert.equal(
    (await core.runCheck(f.workspace, f.store, 'task', 'test')).exitCode,
    7,
  );
  const v = await core.verifyTask(f.workspace, f.store, 'task');
  assert.equal(v.condition, 'failed');
  assert.notEqual(v.phase, 'verified');
});
test('evidence logs cannot be edited into passed after run', async () => {
  const f = await fixture();
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  const run = await core.runCheck(f.workspace, f.store, 'task', 'test');
  await writeFile(
    path.join(f.store.root, 'runs', run.runId, 'stdout.log'),
    'forged',
  );
  assert.notEqual(
    (await core.verifyTask(f.workspace, f.store, 'task')).phase,
    'verified',
  );
});
test('tool output change invalidates prior run without config changes', async () => {
  const f = await fixture();
  const version = path.join(f.root, 'version.txt');
  await writeFile(version, 'v1');
  f.definition.checks[0]!.tools = [
    {
      id: 'version',
      argv: [
        process.execPath,
        '-e',
        `console.log(require('node:fs').readFileSync(${JSON.stringify(version)},'utf8'))`,
      ],
    },
  ];
  await writeFile(
    path.join(f.ws, 'workspace.yaml'),
    JSON.stringify(f.definition),
  );
  const ws = await core.resolveWorkspace(f.ws);
  await core.createTask(ws, f.store, f.input);
  await core.prepareTask(ws, f.store, 'task');
  await core.runCheck(ws, f.store, 'task', 'test');
  await writeFile(version, 'v2');
  assert.notEqual(
    (await core.verifyTask(ws, f.store, 'task')).phase,
    'verified',
  );
});
test('catalog governance changes invalidate selected snapshot', async () => {
  const f = await fixture();
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  const cat = await f.store.readCatalog();
  await core.annotateCatalog(f.workspace, f.store, {
    schemaVersion: 1,
    workspaceId: 'one',
    sourceId: 'docs',
    documentId: f.doc.documentId,
    expectedRevision: cat.revision,
    fields: {
      dependencies: [
        {
          workspaceId: 'one',
          sourceId: 'docs',
          documentId: 'missing-dependency',
        },
      ],
    },
    reason: 'new dependency',
  });
  assert.equal(
    (await core.prepareTask(f.workspace, f.store, 'task')).ready,
    false,
  );
});
test('run artifact symlink is rejected without reading target', async () => {
  const f = await fixture(
    'one',
    "require('node:fs').symlinkSync('/etc/passwd',process.env.AI2WORKSPACE_RUN_DIR+'/artifacts/report.json')",
    { artifacts: ['report.json'] },
  );
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  assert.equal(
    (await core.runCheck(f.workspace, f.store, 'task', 'test')).result,
    'failed',
  );
});
test('source rebinding with identical body still requires baseline review', async () => {
  const f = await fixture();
  await core.createTask(f.workspace, f.store, {
    ...f.input,
    projectIds: [],
    checkIds: [],
    acceptance: [{ id: 'ac', kind: 'manual' }],
  });
  await core.prepareTask(f.workspace, f.store, 'task');
  const another = path.join(f.root, 'another');
  await mkdir(another);
  await writeFile(path.join(another, 'guide.md'), '# Guide\nOriginal\n');
  await writeFile(path.join(another, 'AGENTS.md'), '# Rules\nStay scoped\n');
  await writeFile(
    path.join(f.ws, 'workspace.local.yaml'),
    JSON.stringify({ schemaVersion: 1, bindings: { repo: another } }),
  );
  const rebound = await core.resolveWorkspace(f.ws);
  assert.equal((await core.prepareTask(rebound, f.store, 'task')).ready, false);
});
test('temporary incomplete run never appears as passed after reopened store', async () => {
  const f = await fixture();
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  const dir = path.join(
    f.store.root,
    'runs',
    'run_00000000-0000-0000-0000-000000000000',
  );
  await mkdir(dir, { recursive: true });
  await writeFile(path.join(dir, 'started.json'), '{"claimed":"passed"}');
  const reopened = await core.openSpaceStore(f.workspace);
  assert.equal(
    (await core.verifyTask(f.workspace, reopened, 'task')).reusableRunIds
      .length,
    0,
  );
  assert.equal(
    await readFile(path.join(f.repo, 'app.txt'), 'utf8'),
    'initial user changes',
  );
});
test('remote lock cannot be recovered just by guessing absent PID', async () => {
  const f = await fixture();
  await writeFile(
    path.join(f.store.root, 'space.lock'),
    JSON.stringify({ pid: 2147483647, host: 'another-machine' }),
  );
  await assert.rejects(
    f.store.recoverLock(),
    (e: unknown) => e instanceof core.CoreError && e.exitCode === 5,
  );
});
test('timeout reaps detached owned child before returning', async () => {
  const f = await fixture(
    'one',
    "require('node:child_process').spawn(process.execPath,['-e',\"setInterval(()=>{},1000)\"],{stdio:'ignore'}).once('spawn',function(){console.log(this.pid)});setInterval(()=>{},1000)",
    { timeoutMs: 200 },
  );
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  const r = await core.runCheck(f.workspace, f.store, 'task', 'test');
  assert.equal(r.timedOut, true);
  const pid = Number(
    (
      await readFile(
        path.join(f.store.root, 'runs', r.runId, 'stdout.log'),
        'utf8',
      )
    ).trim(),
  );
  assert(pid > 0);
  await new Promise((resolve) => setTimeout(resolve, 30));
  assert.throws(() => process.kill(pid, 0));
});
test('same task IDs in independent spaces never share runs', async () => {
  const a = await fixture('one');
  const b = await fixture('two');
  for (const f of [a, b]) {
    await core.createTask(f.workspace, f.store, f.input);
    await core.prepareTask(f.workspace, f.store, 'task');
  }
  await core.runCheck(a.workspace, a.store, 'task', 'test');
  assert.equal(
    (await core.verifyTask(b.workspace, b.store, 'task')).reusableRunIds.length,
    0,
  );
  assert.equal((await b.store.readTask('task')).workspaceId, 'two');
});
test('hidden engineering inputs participate in candidate fingerprint', async () => {
  const f = await fixture();
  await mkdir(path.join(f.repo, '.config'));
  await writeFile(path.join(f.repo, '.config', 'build.json'), 'first');
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  await core.runCheck(f.workspace, f.store, 'task', 'test');
  await writeFile(path.join(f.repo, '.config', 'build.json'), 'changed');
  assert.notEqual(
    (await core.verifyTask(f.workspace, f.store, 'task')).phase,
    'verified',
  );
});
test('explicit hidden instruction is selected while private paths remain excluded', async () => {
  const f = await fixture();
  await mkdir(path.join(f.repo, '.agents'));
  await writeFile(
    path.join(f.repo, '.agents', 'rules.md'),
    '# Private rules folder\nExplicit instruction',
  );
  f.definition.sources[1]!.include = ['.agents/rules.md'];
  await writeFile(
    path.join(f.ws, 'workspace.yaml'),
    JSON.stringify(f.definition),
  );
  const ws = await core.resolveWorkspace(f.ws);
  await core.createTask(ws, f.store, f.input);
  await core.prepareTask(ws, f.store, 'task');
  assert(
    (await core.exportContext(ws, f.store, 'task')).documents.some(
      (d) => d.relativePath === '.agents/rules.md',
    ),
  );
});
test('frontmatter confirmation cannot override governance and conflicts are reported', async () => {
  const f = await fixture();
  await writeFile(
    path.join(f.repo, 'guide.md'),
    '---\nstatus: confirmed\ntype: requirement\n---\n# Not confirmed by user',
  );
  const s = await core.scanSources(f.workspace, f.store);
  assert.equal(
    s.documents.find((d) => d.relativePath === 'guide.md')?.metadata.status,
    'unclassified',
  );
  assert(s.issues.some((i) => i.code === 'metadata_conflict'));
});
test('explicit imported identity mapping preserves ID across a move', async () => {
  const f = await fixture();
  await writeFile(
    path.join(f.repo, 'document-ids.json'),
    JSON.stringify([{ id: 'legacy', path: 'old.md' }]),
  );
  await writeFile(path.join(f.repo, 'old.md'), '# Movable');
  const before = await core.scanSources(f.workspace, f.store);
  assert.equal(
    before.documents.find((d) => d.relativePath === 'old.md')?.documentId,
    'legacy',
  );
  await (
    await import('node:fs/promises')
  ).rename(path.join(f.repo, 'old.md'), path.join(f.repo, 'new.md'));
  await writeFile(
    path.join(f.repo, 'document-ids.json'),
    JSON.stringify([{ id: 'legacy', path: 'new.md' }]),
  );
  const after = await core.scanSources(f.workspace, f.store);
  assert.equal(
    after.documents.find((d) => d.relativePath === 'new.md')?.documentId,
    'legacy',
  );
});
test('required metadata conflict blocks readiness without changing original body', async () => {
  const f = await fixture();
  const body = '---\nstatus: confirmed\n---\n# Conflicting';
  await writeFile(path.join(f.repo, 'guide.md'), body);
  await core.createTask(f.workspace, f.store, f.input);
  assert.equal(
    (await core.prepareTask(f.workspace, f.store, 'task')).ready,
    false,
  );
  assert.equal(await readFile(path.join(f.repo, 'guide.md'), 'utf8'), body);
});
test('explicit confirmation cannot be skipped when same document was selected as instruction', async () => {
  const f = await fixture();
  const rules = f.scan.documents.find((d) => d.sourceId === 'rules')!;
  f.input.inputs = [
    {
      workspaceId: 'one',
      sourceId: 'rules',
      documentId: rules.documentId,
      purpose: 'explicit confirmed rule',
      required: true,
      requireConfirmed: true,
    },
  ];
  await core.createTask(f.workspace, f.store, f.input);
  const p = await core.prepareTask(f.workspace, f.store, 'task');
  assert.equal(p.ready, false);
  assert(p.issues.some((i) => i.code === 'confirmation_required'));
});
test('required reference upgrades an earlier optional reference and its dependencies', async () => {
  const f = await fixture();
  const cat = await f.store.readCatalog();
  await core.annotateCatalog(f.workspace, f.store, {
    schemaVersion: 1,
    workspaceId: 'one',
    sourceId: 'docs',
    documentId: f.doc.documentId,
    expectedRevision: cat.revision,
    fields: {
      dependencies: [
        { workspaceId: 'one', sourceId: 'docs', documentId: 'absent' },
      ],
    },
    reason: 'dependency',
  });
  f.input.inputs = [
    { ...f.input.inputs[0]!, required: false },
    { ...f.input.inputs[0]!, required: true },
  ];
  await core.createTask(f.workspace, f.store, f.input);
  assert.equal(
    (await core.prepareTask(f.workspace, f.store, 'task')).ready,
    false,
  );
});
test('missing snapshot content prevents verification and completion', async () => {
  const f = await fixture();
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  const run = await core.runCheck(f.workspace, f.store, 'task', 'test');
  const ctx = await core.exportContext(f.workspace, f.store, 'task');
  await core.createHandoff(f.workspace, f.store, 'task', {
    schemaVersion: 1,
    taskId: 'task',
    contractRevision: 1,
    snapshotId: ctx.snapshotId,
    completedClaims: [{ text: 'checked', evidenceRunIds: [run.runId] }],
    openIssues: [],
    nextSteps: [],
    externalActions: [],
  });
  await (
    await import('node:fs/promises')
  ).unlink(path.join(f.store.root, ctx.documents[0]!.objectPath));
  const v = await core.verifyTask(f.workspace, f.store, 'task');
  assert.notEqual(v.condition, 'clear');
  await assert.rejects(core.completeTask(f.workspace, f.store, 'task'));
});
test('missing initial difference object prevents evidence reuse', async () => {
  const f = await fixture();
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  await core.runCheck(f.workspace, f.store, 'task', 'test');
  const origin = JSON.parse(
    await readFile(
      path.join(f.store.root, 'tasks', 'task', 'origin.json'),
      'utf8',
    ),
  ) as { initialFiles: { path: string; objectPath: string }[] };
  const initial = origin.initialFiles.find((i) => i.path === 'app.txt')!;
  await (
    await import('node:fs/promises')
  ).unlink(path.join(f.store.root, initial.objectPath));
  assert.notEqual(
    (await core.verifyTask(f.workspace, f.store, 'task')).condition,
    'clear',
  );
});
test('unselected delivery and acceptance projects cannot bypass candidate coverage', async () => {
  for (const which of ['delivery', 'acceptance', 'output']) {
    const f = await fixture();
    f.definition.projects.push({
      id: 'out',
      binding: 'repo',
      inputExclude: [],
    });
    await writeFile(
      path.join(f.ws, 'workspace.yaml'),
      JSON.stringify(f.definition),
    );
    const ws = await core.resolveWorkspace(f.ws);
    if (which === 'delivery') {
      f.input.delivery.artifacts = [{ projectId: 'out', path: 'report.txt' }];
    } else if (which === 'acceptance') {
      f.input.acceptance = [
        {
          id: 'ac',
          kind: 'artifact',
          artifact: { projectId: 'out', path: 'report.txt' },
        },
      ];
    } else {
      f.input.scope.outputs = [{ projectId: 'out', path: 'report.txt' }];
    }
    await assert.rejects(
      core.createTask(ws, f.store, f.input),
      (e: unknown) => e instanceof core.CoreError && e.exitCode === 2,
    );
  }
});
test('completed phase preserves history while persisting current applicability drift', async () => {
  const f = await fixture();
  await core.createTask(f.workspace, f.store, f.input);
  const p = await core.prepareTask(f.workspace, f.store, 'task');
  await core.runCheck(f.workspace, f.store, 'task', 'test');
  await core.createHandoff(f.workspace, f.store, 'task', {
    schemaVersion: 1,
    taskId: 'task',
    contractRevision: 1,
    snapshotId: p.snapshotId!,
    completedClaims: [],
    openIssues: [],
    nextSteps: [],
    externalActions: [],
  });
  await core.completeTask(f.workspace, f.store, 'task');
  await writeFile(path.join(f.repo, 'guide.md'), '# New applicability');
  await core.verifyTask(f.workspace, f.store, 'task');
  const state = await f.store.readTask('task');
  assert.equal(state.phase, 'complete');
  assert.equal(state.condition, 'needs_rebaseline');
  assert(state.issues.some((i) => i.code === 'basis_changed'));
});
test('optional manual acceptance does not block required automatic evidence', async () => {
  const f = await fixture();
  f.input.acceptance.push({
    id: 'optional-manual',
    kind: 'manual',
    required: false,
  });
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  await core.runCheck(f.workspace, f.store, 'task', 'test');
  const v = await core.verifyTask(f.workspace, f.store, 'task');
  assert.equal(v.phase, 'verified');
  assert.equal(v.condition, 'clear');
  assert.equal(
    v.acceptance.find((a) => a.id === 'optional-manual')?.satisfied,
    false,
  );
});
test('optional check missing environment does not block unrelated required acceptance', async () => {
  const f = await fixture();
  const required = f.definition.checks[0]!;
  f.definition.checks.push({
    ...required,
    id: 'optional',
    required: false,
    tools: [{ id: 'absent', argv: ['ai2workspace-no-such-tool'] }],
  });
  await writeFile(
    path.join(f.ws, 'workspace.yaml'),
    JSON.stringify(f.definition),
  );
  const ws = await core.resolveWorkspace(f.ws);
  f.input.checkIds.push('optional');
  await core.createTask(ws, f.store, f.input);
  await core.prepareTask(ws, f.store, 'task');
  await core.runCheck(ws, f.store, 'task', 'test');
  assert.equal((await core.verifyTask(ws, f.store, 'task')).phase, 'verified');
});
test('killed command owner cannot recover lock while its owned process group survives', async () => {
  const { spawn } = await import('node:child_process');
  const marker = path.join(
    await mkdtemp(path.join(tmpdir(), 'ai2-owned-')),
    'pid',
  );
  const f = await fixture(
    'one',
    `require('node:fs').writeFileSync(${JSON.stringify(marker)},String(process.pid));setInterval(()=>{},1000)`,
    { timeoutMs: 30000 },
  );
  await core.createTask(f.workspace, f.store, f.input);
  await core.prepareTask(f.workspace, f.store, 'task');
  const moduleUrl = new URL('../src/index.js', import.meta.url).href;
  const worker = spawn(
    process.execPath,
    [
      '--input-type=module',
      '-e',
      `const c=await import(${JSON.stringify(moduleUrl)});const w=await c.resolveWorkspace(${JSON.stringify(f.ws)});const s=await c.openSpaceStore(w);await c.runCheck(w,s,'task','test');`,
    ],
    { stdio: 'ignore' },
  );
  let targetPid = 0;
  let groups: number[] = [];
  try {
    for (let n = 0; n < 300; n++) {
      try {
        targetPid = Number(await readFile(marker, 'utf8'));
        if (targetPid) {
          break;
        }
      } catch {}
      await new Promise((r) => setTimeout(r, 10));
    }
    assert(targetPid > 0);
    const lock = JSON.parse(
      await readFile(path.join(f.store.root, 'space.lock'), 'utf8'),
    ) as { ownedGroups?: { pgid: number }[] };
    groups = lock.ownedGroups?.map((g) => g.pgid) ?? [targetPid];
    worker.kill('SIGKILL');
    await new Promise<void>((resolve) => worker.once('close', () => resolve()));
    assert.doesNotThrow(() => process.kill(targetPid, 0));
    await assert.rejects(
      f.store.recoverLock(),
      (e: unknown) => e instanceof core.CoreError && e.exitCode === 5,
    );
  } finally {
    worker.kill('SIGKILL');
    for (const group of groups) {
      try {
        process.kill(-group, 'SIGKILL');
      } catch {}
    }
    if (targetPid) {
      try {
        process.kill(targetPid, 'SIGKILL');
      } catch {}
    }
  }
  for (let n = 0; n < 100; n++) {
    try {
      await f.store.recoverLock();
      break;
    } catch {
      await new Promise((r) => setTimeout(r, 10));
    }
  }
  assert((await core.scanSources(f.workspace, f.store)).documents.length > 0);
});
test('indexed assets retain exact revision bytes when source changes or disconnects', async () => {
  const f = await fixture();
  await writeFile(path.join(f.repo, 'image.png'), Buffer.from([1, 2, 3]));
  await writeFile(
    path.join(f.repo, 'guide.md'),
    '# Guide\n![Image](image.png)',
  );
  const a = await core.scanSources(f.workspace, f.store);
  await writeFile(path.join(f.repo, 'image.png'), Buffer.from([9]));
  const b = await core.scanSources(f.workspace, f.store);
  assert.notEqual(a.sourceRevision, b.sourceRevision);
  delete f.workspace.bindings.repo;
  assert.deepEqual(
    (
      await core.readIndexedAsset(
        f.workspace,
        f.store,
        a.sourceRevision,
        'docs',
        'image.png',
      )
    ).bytes,
    Buffer.from([1, 2, 3]),
  );
  assert.deepEqual(
    (
      await core.readIndexedAsset(
        f.workspace,
        f.store,
        b.sourceRevision,
        'docs',
        'image.png',
      )
    ).bytes,
    Buffer.from([9]),
  );
});
test('nonhidden runtime files cannot be read as source attachments', async () => {
  const f = await fixture();
  f.workspace.dataDir = path.join(f.repo, 'runtime');
  await mkdir(path.join(f.repo, 'runtime', 'artifacts'), { recursive: true });
  await writeFile(
    path.join(f.repo, 'runtime', 'artifacts', 'private.png'),
    'private',
  );
  await assert.rejects(
    core.readSourceAttachment(
      f.workspace,
      'docs',
      'runtime/artifacts/private.png',
    ),
    (e: unknown) => e instanceof core.CoreError && e.exitCode === 3,
  );
});
test('same logical space ID and data root reject another workspace directory', async () => {
  const f = await fixture();
  const other = path.join(f.root, 'other-ws');
  await mkdir(other);
  await writeFile(
    path.join(other, 'workspace.yaml'),
    JSON.stringify(f.definition),
  );
  await writeFile(
    path.join(other, 'workspace.local.yaml'),
    JSON.stringify({
      schemaVersion: 1,
      bindings: { repo: f.repo },
      dataDir: f.workspace.dataDir,
    }),
  );
  const w = await core.resolveWorkspace(other);
  await assert.rejects(
    core.openSpaceStore(w),
    (e: unknown) => e instanceof core.CoreError && e.exitCode === 5,
  );
});
test('engineering input policy changes require rebaseline even without matching files', async () => {
  const f = await fixture();
  await core.createTask(f.workspace, f.store, f.input);
  const prepared = await core.prepareTask(f.workspace, f.store, 'task');
  const originBefore = await readFile(
    path.join(f.store.root, 'tasks', 'task', 'origin.json'),
    'utf8',
  );
  const run = await core.runCheck(f.workspace, f.store, 'task', 'test');
  assert.equal(run.result, 'passed');
  assert.equal(
    (await core.verifyTask(f.workspace, f.store, 'task')).phase,
    'verified',
  );

  const definition = {
    ...f.definition,
    projects: f.definition.projects.map((project) => ({
      ...project,
      inputExclude: ['**/*.tsbuildinfo'],
    })),
  };
  await writeFile(
    path.join(f.ws, 'workspace.yaml'),
    JSON.stringify(definition),
  );
  const changed = await core.resolveWorkspace(f.ws);
  const verification = await core.verifyTask(changed, f.store, 'task');
  assert.equal(verification.condition, 'needs_rebaseline');
  assert.deepEqual(verification.reusableRunIds, []);
  assert(verification.staleRunIds.includes(run.runId));
  assert.equal((await core.prepareTask(changed, f.store, 'task')).ready, false);
  assert.equal((await f.store.readTask('task')).condition, 'needs_rebaseline');

  const rebased = await core.prepareTask(changed, f.store, 'task', {
    rebaseline: true,
    reason: 'Explicitly exclude the TypeScript incremental build cache',
  });
  assert.equal(rebased.ready, true);
  assert.notEqual(rebased.snapshotId, prepared.snapshotId);
  assert.equal(
    await readFile(
      path.join(f.store.root, 'tasks', 'task', 'origin.json'),
      'utf8',
    ),
    originBefore,
  );
  await writeFile(
    path.join(f.repo, 'generated.tsbuildinfo'),
    'incremental cache',
  );
  assert.equal(
    (await core.runCheck(changed, f.store, 'task', 'test')).result,
    'passed',
  );
  const current = await core.verifyTask(changed, f.store, 'task');
  assert.equal(current.phase, 'verified');
  assert.equal(current.condition, 'clear');
});
