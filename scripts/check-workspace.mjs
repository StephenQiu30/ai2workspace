import { readFile, readdir, lstat } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { createRequire } from 'node:module';

export async function validateRepository(root) {
  const errors = [];
  const require = createRequire(
    new URL('../packages/core/package.json', import.meta.url),
  );
  const { parse } = require('yaml');
  const exists = async (file) => {
    try {
      return !(await lstat(path.join(root, file))).isSymbolicLink();
    } catch {
      return false;
    }
  };
  const json = async (file) => {
    try {
      return JSON.parse(await readFile(path.join(root, file), 'utf8'));
    } catch {
      errors.push('无法读取 JSON：' + file);
      return null;
    }
  };
  for (const file of [
    'AGENTS.md',
    'PROJECT.md',
    'DESIGN.md',
    'code/README.md',
    'code/projects.example.json',
    'code/projects.schema.json',
    'harness/README.md',
    'harness/policy.json',
    'harness/roles.json',
    'harness/standards.md',
    'harness/templates',
    'harness/tasks/acceptance-plan.json',
    'harness/tasks/repository-alignment.json',
    'src/content/document-ids.json',
    'public/media',
    'scripts/setup.mjs',
  ]) {
    if (!(await exists(file))) {
      errors.push('缺少约定路径：' + file);
    }
  }
  for (const file of ['docs', 'templates', 'tools']) {
    if (await exists(file)) {
      errors.push('旧目录造成双重来源：' + file);
    }
  }
  const roles = await json('harness/roles.json');
  const policy = await json('harness/policy.json');
  const plan = await json('harness/tasks/acceptance-plan.json');
  const contract = await json('harness/tasks/repository-alignment.json');
  const pkg = await json('package.json');
  let workspace;
  try {
    workspace = parse(
      await readFile(path.join(root, 'spaces/self/workspace.yaml'), 'utf8'),
    );
  } catch {
    errors.push('自用空间声明不可读');
  }
  if (
    workspace?.harness?.policyPath !== 'harness/policy.json' ||
    workspace?.harness?.projectId !== 'service'
  ) {
    errors.push('自用空间未绑定当前 Harness 策略');
  }
  if (
    policy?.rolesPath !== 'harness/roles.json' ||
    !policy?.requireRoles ||
    !policy?.enforceOutputScope
  ) {
    errors.push('角色和输出约束未启用');
  }
  const ids = roles?.roles?.map((role) => role.id) ?? [];
  if (new Set(ids).size !== ids.length || ids.length !== 9) {
    errors.push('角色身份缺失或重复');
  }
  for (const role of roles?.roles ?? []) {
    if (
      !role.deliverables?.length ||
      !role.reviewRoles?.length ||
      role.reviewRoles.some((id) => !ids.includes(id))
    ) {
      errors.push('角色交付或复核职责不完整：' + role.id);
    }
    if (!(await exists('.agents/skills/workspace-' + role.id + '/SKILL.md'))) {
      errors.push('角色 Skill 缺失：' + role.id);
    }
  }
  const scripts = pkg?.scripts ?? {};
  if (
    scripts['check:workspace'] !== 'node scripts/check-workspace.mjs' ||
    !scripts.test?.includes('pnpm check:workspace')
  ) {
    errors.push('一致性检查未纳入实际测试入口');
  }
  for (const id of new Set([
    ...(policy?.requiredCheckIds ?? []),
    ...(roles?.roles ?? [])
      .filter((role) => contract?.roleIds?.includes(role.id))
      .flatMap((role) => role.checkIds ?? []),
  ])) {
    const check = workspace?.checks?.find((check) => check.id === id);
    if (
      !check?.required ||
      check.argv?.[0] !== 'pnpm' ||
      !scripts[check.argv?.[1]]
    ) {
      errors.push('必需检查与实际脚本脱节：' + id);
    }
    if (!contract?.checkIds?.includes(id)) {
      errors.push('共享实施合同漏掉必需检查：' + id);
    }
  }
  for (const base of ['src/content', 'harness/templates']) {
    const entries = await json(base + '/document-ids.json');
    const seen = new Set();
    for (const entry of entries ?? []) {
      if (seen.has(entry.id) || !(await exists(base + '/' + entry.path))) {
        errors.push('文档身份重复或目标缺失：' + base + '/' + entry.id);
      }
      seen.add(entry.id);
    }
  }
  const expected = Array.from(
    { length: 22 },
    (_, i) => 'AC-' + String(i + 1).padStart(2, '0'),
  );
  const actual = plan?.acceptance?.map((item) => item.id) ?? [];
  if (JSON.stringify(actual) !== JSON.stringify(expected)) {
    errors.push('完整验收计划必须包含 AC-01—AC-22');
  }
  for (const item of plan?.acceptance ?? []) {
    if (!item.evidenceKinds?.length || !item.expected) {
      errors.push('验收缺少通过条件和证据类别：' + item.id);
    }
    if ('status' in item || 'passed' in item) {
      errors.push('共享计划不能手工维护运行状态：' + item.id);
    }
  }
  async function markdownFiles(dir) {
    const result = [];
    for (const entry of await readdir(path.join(root, dir), {
      withFileTypes: true,
    })) {
      const file = dir + '/' + entry.name;
      if (entry.isSymbolicLink()) {
        continue;
      }
      if (entry.isDirectory()) {
        result.push(...(await markdownFiles(file)));
      } else if (file.endsWith('.md') && !file.includes('/archive/')) {
        result.push(file);
      }
    }
    return result;
  }
  const files = [
    'AGENTS.md',
    'PROJECT.md',
    'DESIGN.md',
    'README.md',
    'harness/README.md',
    ...(await markdownFiles('src/content')),
    ...(await markdownFiles('harness/templates')),
  ];
  for (const file of files) {
    const text = await readFile(path.join(root, file), 'utf8');
    for (const link of text.matchAll(/\]\(([^\s)]+)\)/g)) {
      const href = link[1];
      if (/^(?:[a-z]+:|#|\/)/i.test(href)) {
        continue;
      }
      const target = decodeURIComponent(href.split('#')[0]);
      if (!target) {
        continue;
      }
      const resolved = path.resolve(root, path.dirname(file), target);
      if (
        !resolved.startsWith(root + path.sep) ||
        !(await exists(path.relative(root, resolved)))
      ) {
        errors.push('本地链接失效：' + file + ' -> ' + href);
      }
    }
  }
  return errors;
}

if (
  process.argv[1] &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
  const errors = await validateRepository(root);
  if (errors.length) {
    process.stderr.write(errors.join('\n') + '\n');
    process.exitCode = 1;
  } else {
    process.stdout.write(
      '目录、角色、Skill、规则、计划、文档身份、链接与必需检查一致。\n',
    );
  }
}
