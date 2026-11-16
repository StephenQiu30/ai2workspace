import { readFile, realpath } from 'node:fs/promises';
import path from 'node:path';
import { parseDocument } from 'yaml';
import { z } from 'zod';
import { digest, fail, relativePath } from '../storage/file-store.js';
import type {
  ResolvedWorkspace,
  WorkspaceDefinition,
} from '../workspace/types.js';
const id = z.string().regex(/^[a-z0-9][a-z0-9-]{0,63}$/);
const rel = z
  .string()
  .min(1)
  .refine((v) => {
    try {
      relativePath(v);
      return true;
    } catch {
      return false;
    }
  }, 'Must be a relative contained path');
const argv = z
  .array(z.string().refine((v) => !v.includes('\0')))
  .min(1)
  .refine((v) => !!v[0]);
export const workspaceSchema = z.strictObject({
  schemaVersion: z.literal(1),
  id,
  name: z.string().min(1),
  profile: z.enum(['generic', 'development']),
  harness: z.strictObject({ projectId: id, policyPath: rel }).optional(),
  sources: z
    .array(
      z.strictObject({
        id,
        binding: id,
        kind: z.enum(['knowledge', 'reference', 'instruction']),
        projectId: id.optional(),
        include: z.array(rel).min(1),
        exclude: z.array(rel).default([]),
        writePolicy: z.enum(['read-only', 'task-scoped']),
      }),
    )
    .min(1),
  projects: z
    .array(
      z.strictObject({
        id,
        binding: id,
        inputExclude: z.array(rel).default([]),
      }),
    )
    .default([]),
  checks: z
    .array(
      z.strictObject({
        id,
        projectId: id,
        cwd: rel.default('.'),
        argv,
        timeoutMs: z.number().int().min(1).max(3600000).default(120000),
        required: z.boolean().default(true),
        tools: z.array(z.strictObject({ id, argv })).default([]),
        environment: z
          .strictObject({
            publicNames: z
              .array(z.string().regex(/^[A-Za-z_][A-Za-z0-9_]*$/))
              .default([]),
            privateRefs: z.array(z.string()).default([]),
          })
          .default({ publicNames: [], privateRefs: [] }),
        artifacts: z.array(rel).default([]),
      }),
    )
    .default([]),
});
export const localSchema = z.strictObject({
  schemaVersion: z.literal(1),
  bindings: z.record(id, z.string().min(1)),
  autoScan: z.boolean().default(false),
  dataDir: z.string().optional(),
});
async function yaml(file: string): Promise<unknown> {
  try {
    const d = parseDocument(await readFile(file, 'utf8'), { uniqueKeys: true });
    if (d.errors.length) {
      fail('invalid_yaml', d.errors.map((e) => e.message).join('; '));
    }
    return d.toJS({ maxAliasCount: 0 });
  } catch (e) {
    if (e instanceof Error && e.name === 'CoreError') {
      throw e;
    }
    fail(
      'invalid_config',
      `Unable to parse ${path.basename(file)}: ${e instanceof Error ? e.message : 'unknown'}`,
    );
  }
}
export async function resolveWorkspace(
  workspaceDir: string,
): Promise<ResolvedWorkspace> {
  const root = await realpath(workspaceDir);
  const definition = validateWorkspaceDefinition(
    await yaml(path.join(root, 'workspace.yaml')),
  );
  const local = localSchema.safeParse(
    await yaml(path.join(root, 'workspace.local.yaml')),
  );
  if (!local.success) {
    fail('invalid_bindings', local.error.message);
  }
  const bindings: Record<string, string> = {};
  const issues: ResolvedWorkspace['issues'] = [];
  for (const binding of new Set(
    [...definition.sources, ...definition.projects].map((x) => x.binding),
  )) {
    const target = local.data.bindings[binding];
    if (!target) {
      issues.push({
        code: 'missing_binding',
        severity: 'error',
        message: `Missing binding ${binding}`,
      });
      continue;
    }
    if (!path.isAbsolute(target)) {
      fail('invalid_binding', 'Local bindings must be absolute');
    }
    try {
      bindings[binding] = await realpath(target);
    } catch {
      issues.push({
        code: 'missing_binding',
        severity: 'error',
        message: `Unavailable binding ${binding}`,
      });
    }
  }
  const dataDir = path.resolve(
    root,
    local.data.dataDir ?? '.local/ai2workspace',
  );
  return {
    workspaceDir: root,
    dataDir,
    definition,
    bindings,
    bindingRevision: digest(bindings),
    issues,
  };
}

export function validateWorkspaceDefinition(
  input: unknown,
): WorkspaceDefinition {
  const parsed = workspaceSchema.safeParse(input);
  if (!parsed.success) {
    fail('invalid_config', parsed.error.message);
  }
  const definition = parsed.data;
  if (
    definition.harness &&
    !definition.projects.some((p) => p.id === definition.harness!.projectId)
  ) {
    fail('unknown_project', 'Harness policy names an unknown project');
  }
  for (const group of [
    definition.sources,
    definition.projects,
    definition.checks,
  ]) {
    if (new Set(group.map((x) => x.id)).size !== group.length) {
      fail('duplicate_id', 'Duplicate logical ID');
    }
  }
  for (const source of definition.sources) {
    if (
      source.projectId &&
      !definition.projects.some((p) => p.id === source.projectId)
    ) {
      fail('unknown_project', 'Source names an unknown project');
    }
  }
  for (const check of definition.checks) {
    if (!definition.projects.some((p) => p.id === check.projectId)) {
      fail('unknown_project', 'Check names an unknown project');
    }
  }
  return definition;
}
