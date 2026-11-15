import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import { mkdir, writeFile } from 'node:fs/promises';

export async function ensureSelfBindings(root) {
  const directory = resolve(root, 'spaces/self');
  await mkdir(directory, { recursive: true });
  const config = {
    schemaVersion: 1,
    bindings: {
      'knowledge-root': resolve(root, 'docs'),
      'service-root': resolve(root),
      'templates-root': resolve(root, 'templates'),
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

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const created = await ensureSelfBindings(
    resolve(dirname(fileURLToPath(import.meta.url)), '..'),
  );
  process.stdout.write(
    created ? '已创建自用空间的私有本机绑定。\n' : '已有本机绑定保持不变。\n',
  );
}
