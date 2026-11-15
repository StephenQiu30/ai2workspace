import { randomUUID } from 'node:crypto';
import {
  lstat,
  open,
  readFile,
  realpath,
  rename,
  unlink,
} from 'node:fs/promises';
import path from 'node:path';
import { parseDocument } from 'yaml';
import {
  localSchema,
  validateWorkspaceDefinition,
} from '../workspace/resolve.js';
import {
  digest,
  fail,
  hash,
  maybeJson,
  openSpaceStore,
} from '../storage/file-store.js';
import type {
  ResolvedWorkspace,
  WorkspaceDefinition,
} from '../workspace/types.js';

function parse(text: string): unknown {
  const document = parseDocument(text, { uniqueKeys: true });
  if (document.errors.length) {
    return fail('invalid_yaml', '配置必须是有效 YAML');
  }
  try {
    return document.toJS({ maxAliasCount: 0 });
  } catch {
    return fail('invalid_yaml', '配置不支持 YAML 别名');
  }
}

async function regularFile(file: string): Promise<string> {
  const stat = await lstat(file);
  if (!stat.isFile() || stat.isSymbolicLink()) {
    fail('unsafe_configuration', '配置必须是空间目录内的普通文件');
  }
  return readFile(file, 'utf8');
}

export async function updateWorkspaceDefinition(
  workspaceDir: string,
  text: string,
  expectedHash: string,
): Promise<WorkspaceDefinition> {
  if (text.length > 100_000 || !/^[a-f0-9]{64}$/.test(expectedHash)) {
    fail('invalid_configuration_edit', '配置长度或编辑版本无效');
  }
  const definition = validateWorkspaceDefinition(parse(text));
  const root = await realpath(workspaceDir);
  const target = path.join(root, 'workspace.yaml');
  const previous = await regularFile(target);
  const local = localSchema.safeParse(
    parse(await regularFile(path.join(root, 'workspace.local.yaml'))),
  );
  if (!local.success) {
    fail('invalid_bindings', '本机绑定配置需要在本机修复');
  }
  let oldId: unknown;
  try {
    const old = parse(previous);
    if (old && typeof old === 'object' && 'id' in old) {
      oldId = old.id;
    }
  } catch {
    // A catalog identity can recover a declaration with broken YAML syntax.
  }
  const catalog = await maybeJson<{ workspaceId?: string }>(
    path.join(root, 'catalog.json'),
  );
  const knownId = catalog?.workspaceId ?? oldId;
  if (
    typeof knownId !== 'string' ||
    !/^[a-z0-9][a-z0-9-]{0,63}$/.test(knownId)
  ) {
    fail('unknown_space_identity', '无法确定原空间身份，需要在本机明确修复', 3);
  }
  if (knownId !== definition.id) {
    fail('immutable_space_identity', '已有空间 ID 不可变更', 5);
  }
  const lockContext: ResolvedWorkspace = {
    workspaceDir: root,
    dataDir: path.resolve(root, local.data.dataDir ?? '.local/ai2workspace'),
    definition,
    bindings: {},
    bindingRevision: digest({}),
    issues: [],
  };
  const store = await openSpaceStore(lockContext);
  await store.withLock(async () => {
    if (hash(await regularFile(target)) !== expectedHash) {
      fail('revision_conflict', '配置已被其他操作更新，请刷新后重试', 5);
    }
    const temporary = path.join(root, `.workspace-${randomUUID()}.tmp`);
    try {
      const file = await open(temporary, 'wx', 0o600);
      try {
        await file.writeFile(text);
        await file.sync();
      } finally {
        await file.close();
      }
      await rename(temporary, target);
    } finally {
      await unlink(temporary).catch(() => {});
    }
  });
  return definition;
}
