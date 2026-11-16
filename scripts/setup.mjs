import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import { mkdir, writeFile, readFile, rename, unlink } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { randomUUID } from 'node:crypto';

export async function ensureSelfBindings(root) {
  const directory = resolve(root, 'spaces/self');
  await mkdir(directory, { recursive: true });
  const config = {
    schemaVersion: 1,
    bindings: {
      'knowledge-root': resolve(root, 'src/content'),
      'service-root': resolve(root),
      'templates-root': resolve(root, 'harness/templates'),
    },
    autoScan: false,
    dataDir: '.local',
  };
  try {
    await writeFile(
      resolve(directory, 'workspace.local.yaml'),
      JSON.stringify(config, null, 2) + '\n',
      { flag: 'wx', mode: 0o600 },
    );
    return true;
  } catch (error) {
    if (error.code === 'EEXIST') {
      return false;
    }
    throw error;
  }
}

// Explicit, locked migration of only the two known old default bindings.
// Custom paths, comments, dataDir and every historical task asset are retained.
export async function migrateSelfBindings(root) {
  const directory = resolve(root, 'spaces/self');
  const { resolveWorkspace, openSpaceStore } =
    await import('../packages/core/dist/workspace/index.js');
  const require = createRequire(
    new URL('../packages/core/package.json', import.meta.url),
  );
  const { parseDocument } = require('yaml');
  const workspace = await resolveWorkspace(directory);
  const store = await openSpaceStore(workspace);
  return store.withLock(async () => {
    const target = resolve(directory, 'workspace.local.yaml');
    const document = parseDocument(await readFile(target, 'utf8'));
    if (document.errors.length) {
      throw Error('Invalid local bindings');
    }
    let changed = false;
    for (const [binding, oldPath, nextPath] of [
      ['knowledge-root', 'docs', 'src/content'],
      ['templates-root', 'templates', 'harness/templates'],
    ]) {
      if (document.getIn(['bindings', binding]) === resolve(root, oldPath)) {
        document.setIn(['bindings', binding], resolve(root, nextPath));
        changed = true;
      }
    }
    if (!changed) {
      return false;
    }
    const temporary = resolve(directory, '.bindings-' + randomUUID() + '.tmp');
    try {
      await writeFile(temporary, document.toString(), {
        flag: 'wx',
        mode: 0o600,
      });
      await rename(temporary, target);
    } finally {
      await unlink(temporary).catch(() => {});
    }
    return true;
  });
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
  if (process.argv.includes('--migrate-layout')) {
    const migrated = await migrateSelfBindings(root);
    process.stdout.write(
      migrated
        ? '已迁移旧默认资料绑定，运行历史保持不变。\n'
        : '无需迁移；自定义绑定保持不变。\n',
    );
  } else {
    const created = await ensureSelfBindings(root);
    process.stdout.write(
      created ? '已创建自用空间的私有本机绑定。\n' : '已有本机绑定保持不变。\n',
    );
  }
}
