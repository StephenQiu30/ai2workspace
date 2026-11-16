import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const appRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '..',
);
const buildRoot = path.join(appRoot, '.next');
const runtimeRoots = [
  path.resolve(appRoot, '../../spaces'),
  path.resolve(appRoot, '../../knowledge'),
  path.resolve(appRoot, '../../src/content'),
  path.resolve(appRoot, '../../harness'),
  path.resolve(appRoot, '../../code'),
];
if (process.env.AI2WORKSPACE_SPACES_DIR) {
  runtimeRoots.push(path.resolve(process.env.AI2WORKSPACE_SPACES_DIR));
}
const manifests = (
  await readdir(buildRoot, { recursive: true, withFileTypes: true })
).filter((entry) => entry.isFile() && entry.name.endsWith('.nft.json'));
assert.ok(manifests.length > 0, '请先完成 Web production build');
let privateEntries = 0;
for (const entry of manifests) {
  const file = path.join(entry.parentPath, entry.name);
  const manifest = JSON.parse(await readFile(file, 'utf8'));
  const roots = [
    { path: '.', files: manifest.files ?? [] },
    ...(manifest.additionalRoots ?? []),
  ];
  for (const root of roots) {
    for (const relative of root.files) {
      const absolute = path.resolve(path.dirname(file), root.path, relative);
      const privateName = /^(?:workspace\.local\.yaml|\.env(?:\..*)?)$/.test(
        path.basename(absolute),
      );
      if (
        privateName ||
        absolute.split(path.sep).includes('.local') ||
        runtimeRoots.some(
          (directory) =>
            absolute === directory || absolute.startsWith(directory + path.sep),
        )
      ) {
        privateEntries++;
      }
    }
  }
}
assert.equal(
  privateEntries,
  0,
  '构建追踪不能包含空间配置、来源正文或私有运行数据',
);
process.stdout.write(
  `Checked ${manifests.length} build trace manifests: no runtime workspace files.\n`,
);
