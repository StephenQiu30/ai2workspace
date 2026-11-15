import {
  mkdtempSync,
  mkdirSync,
  writeFileSync,
  existsSync,
  symlinkSync,
} from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export function createTestRegistry() {
  const registry =
    process.env.AI2WORKSPACE_TEST_REGISTRY ??
    mkdtempSync(path.join(os.tmpdir(), 'ai2workspace-web-'));
  process.env.AI2WORKSPACE_TEST_REGISTRY = registry;
  if (existsSync(path.join(registry, 'self'))) {
    return registry;
  }
  const root = path.resolve(
    path.dirname(fileURLToPath(import.meta.url)),
    '../../..',
  );
  const notes = path.join(registry, 'fixture-notes');
  mkdirSync(notes);
  writeFileSync(
    path.join(notes, 'one.md'),
    '# Second space\n\nFixtureOnlySecondSpace\n\n## Choice\n\n{globalThis.__markdownExecuted = true}\n\n<script>globalThis.__markdownExecuted = true</script>\n\n![diagram](diagram.png)\n',
  );
  writeFileSync(
    path.join(notes, 'diagram.png'),
    Buffer.from(
      'iVBORw0KGgoAAAANSUhEUgAAAEAAAAAgCAYAAACinX6EAAAAWUlEQVR4nO3QsQ0AMAjAME7n83KGh2bwHmV29/1sdIDWAB2gNUAHaA3QAVoDdIDWAB2gNUAHaA3QAVoDdIDWAB2gNUAHaA3QAVoDdIDWAB2gNUAHaA3QAdoBeBT5HumyqL8AAAAASUVORK5CYII=',
      'base64',
    ),
  );
  writeFileSync(
    path.join(notes, 'document-ids.json'),
    JSON.stringify([{ id: 'requirement-01', path: 'one.md' }]),
  );
  symlinkSync(path.join(root, 'AGENTS.md'), path.join(notes, 'escape.png'));
  for (const [key, binding] of [
    ['self', path.join(root, 'docs')],
    ['second', notes],
  ]) {
    const dir = path.join(registry, key);
    mkdirSync(dir);
    writeFileSync(
      path.join(dir, 'workspace.yaml'),
      `schemaVersion: 1\nid: ${key}\nname: ${key === 'self' ? 'ai2workspace 自用' : '第二空间'}\nprofile: generic\nsources:\n  - id: knowledge\n    binding: notes\n    kind: knowledge\n    include: ['**/*.md']\n    exclude: []\n    writePolicy: read-only\n${key === 'self' ? "  - id: rules\n    binding: rules\n    kind: instruction\n    include: ['AGENTS.md', 'PROJECT.md', 'DESIGN.md']\n    exclude: []\n    writePolicy: read-only\n  - id: templates\n    binding: templates\n    kind: reference\n    include: ['knowledge/*.md', 'workspace/README.md', 'harness/README.md']\n    exclude: []\n    writePolicy: read-only\n" : ''}projects: []\nchecks: []\n`,
    );
    writeFileSync(
      path.join(dir, 'workspace.local.yaml'),
      `schemaVersion: 1\nbindings:\n  notes: ${JSON.stringify(binding)}\n  rules: ${JSON.stringify(root)}\n  templates: ${JSON.stringify(path.join(root, 'templates'))}\nautoScan: false\n`,
    );
  }
  return registry;
}
