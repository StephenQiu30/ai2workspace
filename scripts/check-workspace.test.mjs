import test from 'node:test';
import assert from 'node:assert/strict';
import { cp, mkdir, mkdtemp, readFile, writeFile, rm } from 'node:fs/promises';
import path from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { validateRepository } from './check-workspace.mjs';

async function fixture(t) {
  const root = await mkdtemp(path.join(tmpdir(), 'ai2-layout-'));
  t.after(() => rm(root, { recursive: true, force: true }));
  const source = path.resolve(
    path.dirname(fileURLToPath(import.meta.url)),
    '..',
  );
  for (const file of [
    'AGENTS.md',
    'PROJECT.md',
    'DESIGN.md',
    'README.md',
    'BACKLOG.md',
    'package.json',
    'pnpm-workspace.yaml',
    'src/content',
    'harness',
    'code',
    '.agents/skills',
    'scripts/setup.mjs',
    'public/media',
    'packages/core/src',
    'apps/cli/src',
    'apps/web/README.md',
    'spaces/self/workspace.yaml',
  ]) {
    const target = path.join(root, file);
    await mkdir(path.dirname(target), { recursive: true });
    await cp(path.join(source, file), target, { recursive: true });
  }
  assert.deepEqual(
    await validateRepository(root),
    [],
    'Fixture must reproduce a valid public checkout',
  );
  return root;
}

test('alignment check detects a reintroduced duplicate content root and missing role skill', async (t) => {
  const root = await fixture(t);
  await mkdir(path.join(root, 'docs'));
  await rm(path.join(root, '.agents/skills/workspace-fe'), { recursive: true });
  const errors = await validateRepository(root);
  assert.ok(errors.some((message) => message.includes('旧目录')));
  assert.ok(errors.some((message) => message.includes('角色 Skill 缺失')));
});

test('alignment check detects missing mandatory check, broken stable ID and duplicate live status', async (t) => {
  const root = await fixture(t);
  const contractPath = path.join(
    root,
    'harness/tasks/repository-alignment.json',
  );
  const contract = JSON.parse(await readFile(contractPath, 'utf8'));
  contract.checkIds = [];
  await writeFile(contractPath, JSON.stringify(contract));
  const idsPath = path.join(root, 'src/content/document-ids.json');
  const ids = JSON.parse(await readFile(idsPath, 'utf8'));
  ids[0].path = 'missing.md';
  await writeFile(idsPath, JSON.stringify(ids));
  const planPath = path.join(root, 'harness/tasks/acceptance-plan.json');
  const plan = JSON.parse(await readFile(planPath, 'utf8'));
  plan.acceptance[0].status = 'passed';
  await writeFile(planPath, JSON.stringify(plan));
  const errors = await validateRepository(root);
  assert.ok(errors.some((message) => message.includes('漏掉必需检查')));
  assert.ok(errors.some((message) => message.includes('文档身份')));
  assert.ok(errors.some((message) => message.includes('不能手工维护运行状态')));
});
