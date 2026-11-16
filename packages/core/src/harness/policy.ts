import { readFile } from 'node:fs/promises';
import { z } from 'zod';
import type { ResolvedWorkspace } from '../workspace/types.js';
import type { TaskInput, Fingerprint } from './types.js';
import type { Issue } from '../shared/types.js';
import { contained, digest, relativePath } from '../storage/file-store.js';

const id = z.string().regex(/^[a-z0-9][a-z0-9-]{0,63}$/);
const relative = z
  .string()
  .min(1)
  .refine((value) => {
    try {
      relativePath(value);
      return !value.includes('\\');
    } catch {
      return false;
    }
  });
export const rolesSchema = z.strictObject({
  schemaVersion: z.literal(1),
  roles: z
    .array(
      z.strictObject({
        id,
        name: z.string().min(1),
        ownedPaths: z.array(relative),
        deliverables: z.array(z.string().min(1)).min(1),
        reviewRoles: z.array(id),
        checkIds: z.array(id).optional(),
      }),
    )
    .min(1),
});
export const policySchema = z.strictObject({
  schemaVersion: z.literal(1),
  rolesPath: relative,
  requiredCheckIds: z.array(id).min(1),
  requireRoles: z.boolean(),
  enforceOutputScope: z.boolean(),
});
type Policy = z.infer<typeof policySchema>;
type Roles = z.infer<typeof rolesSchema>;
export interface PolicySnapshot {
  hash: string;
  projectId: string;
  policy: Policy;
  roles: Roles['roles'];
}
export interface LoadedPolicy {
  hash?: string;
  policy?: Policy;
  roles?: Roles['roles'];
  projectId?: string;
  issues: Issue[];
}
function issue(code: string, message: string, file?: string): Issue {
  return { code, severity: 'error', message, ...(file ? { path: file } : {}) };
}
function owns(paths: string[], file: string): boolean {
  return paths.some((owned) =>
    owned.endsWith('/') ? file.startsWith(owned) : file === owned,
  );
}

// The current project explicitly binds this policy; legacy readers remain compatible.
// These checks reject acceptance; they are not an operating-system sandbox.
export async function loadHarnessPolicy(
  workspace: ResolvedWorkspace,
): Promise<LoadedPolicy> {
  const reference = workspace.definition.harness;
  if (!reference) {
    return { issues: [] };
  }
  try {
    const project = workspace.definition.projects.find(
      (p) => p.id === reference.projectId,
    );
    const root = project && workspace.bindings[project.binding];
    if (!root) {
      throw Error('Policy project unavailable');
    }
    const policy = policySchema.parse(
      JSON.parse(
        await readFile(await contained(root, reference.policyPath), 'utf8'),
      ),
    );
    const registry = rolesSchema.parse(
      JSON.parse(
        await readFile(await contained(root, policy.rolesPath), 'utf8'),
      ),
    );
    const roleIds = registry.roles.map((r) => r.id);
    if (
      new Set(roleIds).size !== roleIds.length ||
      new Set(policy.requiredCheckIds).size !== policy.requiredCheckIds.length
    ) {
      throw Error('Duplicate policy identity');
    }
    if (
      registry.roles.some((r) =>
        r.reviewRoles.some((reviewer) => !roleIds.includes(reviewer)),
      )
    ) {
      throw Error('Unknown review role');
    }
    if (
      [
        ...policy.requiredCheckIds,
        ...registry.roles.flatMap((role) => role.checkIds ?? []),
      ].some(
        (check) =>
          !workspace.definition.checks.some(
            (c) => c.id === check && c.required,
          ),
      )
    ) {
      throw Error('Required policy check missing or optional');
    }
    return {
      hash: digest({ reference, policy, registry }),
      policy,
      roles: registry.roles,
      projectId: reference.projectId,
      issues: [],
    };
  } catch {
    return {
      issues: [
        issue(
          'harness_policy_unavailable',
          'Harness policy or role registry is invalid or unavailable',
        ),
      ],
    };
  }
}

export function contractPolicyIssues(
  workspace: ResolvedWorkspace,
  contract: TaskInput,
  loaded: LoadedPolicy,
): Issue[] {
  const issues = [...loaded.issues];
  const policy = loaded.policy;
  if (!policy) {
    return issues;
  }
  if (!contract.projectIds.includes(loaded.projectId!)) {
    issues.push(
      issue(
        'policy_project_required',
        'Policy tasks must include the current Workspace project',
      ),
    );
  }
  const selected = contract.roleIds ?? [];
  const requiredChecks = new Set([
    ...policy.requiredCheckIds,
    ...loaded
      .roles!.filter((role) => selected.includes(role.id))
      .flatMap((role) => role.checkIds ?? []),
  ]);
  for (const checkId of requiredChecks) {
    const check = workspace.definition.checks.find((c) => c.id === checkId)!;
    if (!contract.projectIds.includes(check.projectId)) {
      continue;
    }
    if (!contract.checkIds.includes(checkId)) {
      issues.push(
        issue(
          'required_policy_check',
          'Task omits mandatory Harness check: ' + checkId,
        ),
      );
    }
  }
  if (
    new Set(selected).size !== selected.length ||
    selected.some((role) => !loaded.roles!.some((r) => r.id === role))
  ) {
    issues.push(
      issue('invalid_task_roles', 'Task contains duplicate or unknown roles'),
    );
  }
  if (policy.requireRoles && !selected.length) {
    issues.push(
      issue(
        'missing_task_roles',
        'Task must explicitly select responsible roles',
      ),
    );
  }
  const owned = loaded
    .roles!.filter((role) => selected.includes(role.id))
    .flatMap((role) => role.ownedPaths);
  for (const output of contract.scope.outputs) {
    if (!('projectId' in output)) {
      if (policy.enforceOutputScope) {
        issues.push(
          issue(
            'explicit_output_path_required',
            'Policy tasks must declare exact project paths for document edits',
          ),
        );
      }
    } else if (
      output.projectId === loaded.projectId &&
      !owns(owned, output.path)
    ) {
      issues.push(
        issue(
          'role_scope_violation',
          'Declared output is outside the selected roles',
          output.path,
        ),
      );
    }
  }
  return issues;
}

export function outputScopeIssues(
  contract: TaskInput,
  loaded: LoadedPolicy,
  initial: Record<string, Fingerprint>,
  current: Record<string, Fingerprint>,
): Issue[] {
  if (!loaded.policy?.enforceOutputScope) {
    return [];
  }
  const issues: Issue[] = [];
  if (
    Object.keys(initial).some(
      (projectId) => !contract.projectIds.includes(projectId),
    )
  ) {
    issues.push(
      issue(
        'original_project_omitted',
        'Task cannot omit an engineering project covered by its original baseline',
      ),
    );
  }
  for (const projectId of contract.projectIds) {
    if (!initial[projectId]) {
      issues.push(
        issue(
          'missing_origin_baseline',
          'Original task baseline does not cover this project; create an explicit new task',
        ),
      );
      continue;
    }
    const allowed = new Set(
      contract.scope.outputs.flatMap((output) =>
        'projectId' in output && output.projectId === projectId
          ? [output.path]
          : [],
      ),
    );
    const before = new Map(
      initial[projectId]?.files.map((file) => [file.path, file.hash]) ?? [],
    );
    const after = new Map(
      current[projectId]?.files.map((file) => [file.path, file.hash]) ?? [],
    );
    const changed = [...new Set([...before.keys(), ...after.keys()])].filter(
      (file) => before.get(file) !== after.get(file),
    );
    const owned = loaded
      .roles!.filter((role) => contract.roleIds?.includes(role.id))
      .flatMap((role) => role.ownedPaths);
    for (const file of changed) {
      if (!allowed.has(file)) {
        issues.push(
          issue(
            'output_scope_violation',
            'Actual change is outside declared task outputs',
            file,
          ),
        );
      }
      if (projectId === loaded.projectId && !owns(owned, file)) {
        issues.push(
          issue(
            'role_scope_violation',
            'Actual change is outside selected roles',
            file,
          ),
        );
      }
    }
  }
  return issues;
}
