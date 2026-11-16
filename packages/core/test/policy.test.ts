import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, writeFile, readFile, rm } from 'node:fs/promises';
import path from 'node:path';
import { tmpdir } from 'node:os';
import * as core from '../src/index.js';
import type { TaskInput } from '../src/contracts.js';

async function fixture() {
  const root = await mkdtemp(path.join(tmpdir(), 'ai2-policy-'));
  const repo = path.join(root, 'repo');
  const directory = path.join(root, 'workspace');
  await mkdir(path.join(repo, 'src'), { recursive: true });
  await mkdir(path.join(repo, 'harness'));
  await mkdir(directory);
  await writeFile(path.join(repo, 'src/app.ts'), 'export const value = 1;');
  await writeFile(path.join(repo, 'AGENTS.md'), '# Project rules');
  const policy = {
    schemaVersion: 1,
    rolesPath: 'harness/roles.json',
    requiredCheckIds: ['alignment', 'test'],
    requireRoles: true,
    enforceOutputScope: true,
  };
  const roles = {
    schemaVersion: 1,
    roles: [
      {
        id: 'fe',
        name: '前端',
        ownedPaths: ['src/'],
        deliverables: ['实现与自测'],
        reviewRoles: ['qa'],
      },
      {
        id: 'qa',
        name: '质量',
        ownedPaths: ['harness/'],
        deliverables: ['验证'],
        reviewRoles: ['fe'],
      },
    ],
  };
  await writeFile(
    path.join(repo, 'harness/policy.json'),
    JSON.stringify(policy),
  );
  await writeFile(path.join(repo, 'harness/roles.json'), JSON.stringify(roles));
  const definition = {
    schemaVersion: 1,
    id: 'project',
    name: '项目模块',
    profile: 'development',
    harness: { projectId: 'app', policyPath: 'harness/policy.json' },
    sources: [
      {
        id: 'rules',
        binding: 'repo',
        kind: 'instruction',
        include: ['AGENTS.md'],
        exclude: [],
        writePolicy: 'read-only',
      },
    ],
    projects: [{ id: 'app', binding: 'repo', inputExclude: [] }],
    checks: ['alignment', 'test'].map((id) => ({
      id,
      projectId: 'app',
      cwd: '.',
      argv: [process.execPath, '-e', 'console.log("actual check")'],
      required: true,
    })),
  };
  await writeFile(
    path.join(directory, 'workspace.yaml'),
    JSON.stringify(definition),
  );
  await writeFile(
    path.join(directory, 'workspace.local.yaml'),
    JSON.stringify({ schemaVersion: 1, bindings: { repo } }),
  );
  const workspace = await core.resolveWorkspace(directory);
  const store = await core.openSpaceStore(workspace);
  const contract: TaskInput = {
    schemaVersion: 1,
    id: 'change',
    workspaceId: 'project',
    kind: 'implementation',
    goal: 'Modify app within scope',
    roleIds: ['fe'],
    projectIds: ['app'],
    scope: { outputs: [{ projectId: 'app', path: 'src/app.ts' }] },
    inputs: [],
    checkIds: ['alignment', 'test'],
    acceptance: [
      { id: 'checks', checkIds: ['alignment', 'test'], required: true },
    ],
    delivery: {
      artifacts: [{ projectId: 'app', path: 'src/app.ts' }],
      requireHumanAcceptance: false,
    },
  };
  return { root, repo, directory, policy, roles, workspace, store, contract };
}

test('project policy rejects omitted mandatory checks and missing or invalid roles before task creation', async (t) => {
  const f = await fixture();
  t.after(() => rm(f.root, { recursive: true, force: true }));
  for (const input of [
    {
      ...f.contract,
      checkIds: ['test'],
      acceptance: [{ id: 'test', checkIds: ['test'] }],
    },
    { ...f.contract, roleIds: [] },
    { ...f.contract, roleIds: ['unknown'] },
    { ...f.contract, roleIds: ['fe', 'fe'] },
  ]) {
    await assert.rejects(
      core.createTask(f.workspace, f.store, input),
      (error: unknown) =>
        error instanceof core.CoreError && error.exitCode === 3,
    );
  }
  assert.deepEqual(await f.store.readTaskStates(), []);
});

test('declared outputs must belong to selected roles, with directory boundaries respected', async (t) => {
  const f = await fixture();
  t.after(() => rm(f.root, { recursive: true, force: true }));
  for (const output of ['harness/policy.json', 'src-other/app.ts']) {
    await assert.rejects(
      core.createTask(f.workspace, f.store, {
        ...f.contract,
        scope: { outputs: [{ projectId: 'app', path: output }] },
      }),
      /outside the selected roles/,
    );
  }
});

test('role-specific browser checks cannot be omitted by a frontend task', async (t) => {
  const f = await fixture();
  t.after(() => rm(f.root, { recursive: true, force: true }));
  f.workspace.definition.checks.push({
    ...f.workspace.definition.checks[0]!,
    id: 'browser',
  });
  await writeFile(
    path.join(f.repo, 'harness/roles.json'),
    JSON.stringify({
      ...f.roles,
      roles: f.roles.roles.map((role) =>
        role.id === 'fe' ? { ...role, checkIds: ['browser'] } : role,
      ),
    }),
  );
  await assert.rejects(
    core.createTask(f.workspace, f.store, f.contract),
    /mandatory Harness check: browser/,
  );
});

test('real checks verify declared changes but refuse undeclared files and terminal completion', async (t) => {
  const f = await fixture();
  t.after(() => rm(f.root, { recursive: true, force: true }));
  await core.createTask(f.workspace, f.store, f.contract);
  assert.equal(
    (await core.prepareTask(f.workspace, f.store, 'change')).ready,
    true,
  );
  await writeFile(path.join(f.repo, 'src/app.ts'), 'export const value = 2;');
  for (const check of f.contract.checkIds) {
    assert.equal(
      (await core.runCheck(f.workspace, f.store, 'change', check)).result,
      'passed',
    );
  }
  assert.equal(
    (await core.verifyTask(f.workspace, f.store, 'change')).condition,
    'clear',
  );
  await writeFile(path.join(f.repo, 'src/unplanned.ts'), 'unplanned');
  const result = await core.verifyTask(f.workspace, f.store, 'change');
  assert.ok(
    result.issues.some(
      (issue) =>
        issue.code === 'output_scope_violation' &&
        issue.path === 'src/unplanned.ts',
    ),
  );
  await assert.rejects(core.completeTask(f.workspace, f.store, 'change'));
});

test('role registry drift invalidates preparation and old check evidence', async (t) => {
  const f = await fixture();
  t.after(() => rm(f.root, { recursive: true, force: true }));
  await core.createTask(f.workspace, f.store, f.contract);
  const first = await core.prepareTask(f.workspace, f.store, 'change');
  assert.equal(first.ready, true);
  const pinned = await core.exportContext(f.workspace, f.store, 'change');
  assert.deepEqual(pinned.policySnapshot?.policy.requiredCheckIds, [
    'alignment',
    'test',
  ]);
  await core.runCheck(f.workspace, f.store, 'change', 'test');
  f.roles.roles[0]!.deliverables.push('additional requirement');
  await writeFile(
    path.join(f.repo, 'harness/roles.json'),
    JSON.stringify(f.roles),
  );
  const historical = await core.exportContext(f.workspace, f.store, 'change');
  assert.deepEqual(historical.policySnapshot, pinned.policySnapshot);
  const next = await core.prepareTask(f.workspace, f.store, 'change');
  assert.equal(next.ready, false);
  assert.ok(
    next.issues.some((issue) => issue.code === 'harness_policy_changed'),
  );
  const result = await core.verifyTask(f.workspace, f.store, 'change');
  assert.equal(result.condition, 'needs_rebaseline');
  assert.equal(result.reusableRunIds.length, 0);
});

test('explicit rebaseline preserves origin and cannot launder an out-of-scope change', async (t) => {
  const f = await fixture();
  t.after(() => rm(f.root, { recursive: true, force: true }));
  await core.createTask(f.workspace, f.store, f.contract);
  await core.prepareTask(f.workspace, f.store, 'change');
  const origin = await readFile(
    path.join(f.store.root, 'tasks/change/origin.json'),
    'utf8',
  );
  await writeFile(path.join(f.repo, 'src/rogue.ts'), 'outside exact scope');
  await core.prepareTask(f.workspace, f.store, 'change', {
    rebaseline: true,
    reason: 'Explicitly refresh basis',
  });
  assert.equal(
    await readFile(path.join(f.store.root, 'tasks/change/origin.json'), 'utf8'),
    origin,
  );
  const result = await core.verifyTask(f.workspace, f.store, 'change');
  assert.ok(
    result.issues.some((issue) => issue.code === 'output_scope_violation'),
  );
});

test('missing or corrupt policy fails closed and policy paths cannot escape the registered project', async (t) => {
  const f = await fixture();
  t.after(() => rm(f.root, { recursive: true, force: true }));
  await writeFile(path.join(f.repo, 'harness/roles.json'), '{broken');
  await assert.rejects(
    core.createTask(f.workspace, f.store, f.contract),
    /invalid or unavailable/,
  );
  assert.throws(() =>
    core.validateWorkspaceDefinition({
      ...f.workspace.definition,
      harness: { projectId: 'app', policyPath: '../policy.json' },
    }),
  );
  assert.throws(() =>
    core.validateWorkspaceDefinition({
      ...f.workspace.definition,
      harness: { projectId: 'missing', policyPath: 'harness/policy.json' },
    }),
  );
});

test('a prepared task cannot remove its Workspace project to wash away undeclared changes', async (t) => {
  const f = await fixture();
  t.after(() => rm(f.root, { recursive: true, force: true }));
  await core.createTask(f.workspace, f.store, f.contract);
  await core.prepareTask(f.workspace, f.store, 'change');
  await writeFile(path.join(f.repo, 'src/rogue.ts'), 'undeclared');
  const bypass = {
    ...f.contract,
    projectIds: [],
    scope: { outputs: [] },
    checkIds: [],
    acceptance: [{ id: 'optional', required: false }],
    delivery: { artifacts: [], requireHumanAcceptance: false },
  };
  await assert.rejects(
    core.reviseTask(f.workspace, f.store, 'change', {
      contract: bypass,
      expectedRevision: 1,
      reason: 'Remove project',
    }),
    /must include the current Workspace project/,
  );
  const result = await core.verifyTask(f.workspace, f.store, 'change');
  assert.ok(
    result.issues.some((issue) => issue.code === 'output_scope_violation'),
  );
  assert.notEqual(result.condition, 'clear');
});

test('original secondary engineering projects also remain in the audit set', async (t) => {
  const f = await fixture();
  t.after(() => rm(f.root, { recursive: true, force: true }));
  const auxiliary = path.join(f.root, 'auxiliary');
  await mkdir(auxiliary);
  await writeFile(path.join(auxiliary, 'app.txt'), 'original');
  f.workspace.definition.projects.push({
    id: 'aux',
    binding: 'aux',
    inputExclude: [],
  });
  f.workspace.bindings.aux = auxiliary;
  const original = { ...f.contract, projectIds: ['app', 'aux'] };
  await core.createTask(f.workspace, f.store, original);
  await core.prepareTask(f.workspace, f.store, 'change');
  await writeFile(path.join(auxiliary, 'rogue.txt'), 'undeclared');
  await assert.rejects(
    core.reviseTask(f.workspace, f.store, 'change', {
      contract: f.contract,
      expectedRevision: 1,
      reason: 'Remove auxiliary project',
    }),
    /projects are fixed/,
  );
});

test('explicit rebaseline cannot remove a policy from an already governed task', async (t) => {
  const f = await fixture();
  t.after(() => rm(f.root, { recursive: true, force: true }));
  await core.createTask(f.workspace, f.store, f.contract);
  const before = await core.prepareTask(f.workspace, f.store, 'change');
  delete f.workspace.definition.harness;
  const after = await core.prepareTask(f.workspace, f.store, 'change', {
    rebaseline: true,
    reason: 'Drop policy',
  });
  assert.equal(after.ready, false);
  assert.equal(after.snapshotId, before.snapshotId);
  assert.ok(
    after.issues.some((issue) => issue.code === 'harness_policy_removed'),
  );
});
