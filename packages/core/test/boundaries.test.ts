import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// The break this catches: a lower domain acquires executable task/command logic,
// or a caller bypasses its named domain API through the compatibility barrel.
test('domain dependencies point downwards and application callers use named APIs', async () => {
  const repo = path.resolve(
    path.dirname(fileURLToPath(import.meta.url)),
    '../../../..',
  );
  const root = path.join(repo, 'packages/core/src');
  const allowed: Record<string, string[]> = {
    workspace: ['workspace', 'shared', 'storage'],
    knowledge: ['knowledge', 'workspace', 'shared', 'storage'],
    context: ['context', 'knowledge', 'workspace', 'shared', 'storage'],
    harness: [
      'harness',
      'context',
      'knowledge',
      'workspace',
      'shared',
      'storage',
    ],
  };
  async function files(dir: string): Promise<string[]> {
    const result: string[] = [];
    for (const e of await readdir(dir, { withFileTypes: true })) {
      if (
        ['node_modules', '.next', '.source', 'dist', '.test-build'].includes(
          e.name,
        )
      ) {
        continue;
      }
      const p = path.join(dir, e.name);
      if (e.isDirectory()) {
        result.push(...(await files(p)));
      } else if (/\.tsx?$/.test(e.name)) {
        result.push(p);
      }
    }
    return result;
  }
  for (const file of await files(root)) {
    const domain = path.relative(root, file).split(path.sep)[0]!;
    if (!allowed[domain]) {
      continue;
    }
    const source = await readFile(file, 'utf8');
    for (const m of source.matchAll(
      /(?:import|export)\s+(?!type\b)[^;]*?from\s+['"]([^'"]+)['"]/g,
    )) {
      const spec = m[1]!;
      if (!spec.startsWith('.')) {
        continue;
      }
      const target = path
        .relative(root, path.resolve(path.dirname(file), spec))
        .split(path.sep)[0]!;
      assert.ok(
        allowed[domain]!.includes(target),
        `${path.relative(root, file)} imports upward into ${spec}`,
      );
    }
  }
  for (const app of [
    'apps/web/lib',
    'apps/web/app',
    'apps/web/components',
    'apps/cli/src',
  ]) {
    for (const file of await files(path.join(repo, app))) {
      assert.doesNotMatch(
        await readFile(file, 'utf8'),
        /from ['"]@ai2workspace\/core['"]/,
        `${path.relative(repo, file)} must choose a domain API`,
      );
    }
  }
});
