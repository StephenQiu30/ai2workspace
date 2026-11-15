import { randomUUID } from 'node:crypto';
import path from 'node:path';
import { readFile, writeFile } from 'node:fs/promises';
import { z } from 'zod';
import type { ResolvedWorkspace } from '../workspace/types.js';
import type {
  TaskInput,
  TaskState,
  TaskRevisionInput,
  VerificationResult,
  HandoffNotes,
  Checkpoint,
  ResumeResult,
} from '../harness/types.js';
import type { Issue } from '../shared/types.js';
import {
  SpaceStore,
  digest,
  hash,
  atomicJson,
  contained,
  relativePath,
  fail,
  maybeJson,
} from '../storage/file-store.js';
import {
  contextDrift,
  candidates,
  verifySnapshotObjects,
} from './preparation.js';
import { probe } from './checks.js';
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
  });
const key = z.strictObject({
  workspaceId: id,
  sourceId: id,
  documentId: z.string().min(1),
});
const artifact = z.strictObject({ projectId: id, path: rel });
export const taskInputSchema = z.strictObject({
  schemaVersion: z.literal(1),
  id,
  workspaceId: id,
  kind: z.enum(['implementation', 'knowledge-maintenance']),
  goal: z.string().min(1),
  scope: z.strictObject({
    description: z.string().optional(),
    outputs: z
      .array(
        z.union([
          z.strictObject({ sourceId: id, documentId: z.string().min(1) }),
          artifact,
        ]),
      )
      .default([]),
  }),
  projectIds: z.array(id),
  inputs: z.array(
    key.extend({
      section: z.string().optional(),
      purpose: z.string().min(1),
      required: z.boolean(),
      requireConfirmed: z.boolean().optional(),
    }),
  ),
  acceptance: z
    .array(
      z.strictObject({
        id,
        checkIds: z.array(id).optional(),
        kind: z.enum(['automatic', 'manual', 'artifact']).optional(),
        required: z.boolean().optional(),
        artifact: artifact.optional(),
      }),
    )
    .min(1),
  checkIds: z.array(id),
  delivery: z.strictObject({
    artifacts: z.array(artifact),
    requireHumanAcceptance: z.boolean(),
  }),
});
function validateContract(
  workspace: ResolvedWorkspace,
  input: TaskInput,
): TaskInput {
  const parsed = taskInputSchema.safeParse(input);
  if (!parsed.success) {
    fail('invalid_contract', parsed.error.message);
  }
  const c = parsed.data;
  if (
    c.workspaceId !== workspace.definition.id ||
    c.inputs.some((i) => i.workspaceId !== c.workspaceId)
  ) {
    fail('workspace_mismatch', 'Contract contains another space', 5);
  }
  if (
    new Set(c.acceptance.map((a) => a.id)).size !== c.acceptance.length ||
    new Set(c.checkIds).size !== c.checkIds.length ||
    new Set(c.projectIds).size !== c.projectIds.length
  ) {
    fail('duplicate_id', 'Duplicate contract identity');
  }
  for (const projectId of c.projectIds) {
    if (!workspace.definition.projects.some((p) => p.id === projectId)) {
      fail('unknown_project', 'Task project unavailable');
    }
  }
  const artifactProjects = [
    ...c.delivery.artifacts,
    ...c.acceptance.flatMap((a) => (a.artifact ? [a.artifact] : [])),
    ...c.scope.outputs.filter(
      (o): o is { projectId: string; path: string } => 'projectId' in o,
    ),
  ];
  for (const artifact of artifactProjects) {
    if (!c.projectIds.includes(artifact.projectId)) {
      fail(
        'unselected_project',
        'Every delivery, acceptance artifact and project output must belong to a selected project',
      );
    }
  }
  for (const checkId of c.checkIds) {
    const check = workspace.definition.checks.find((c) => c.id === checkId);
    if (!check || !c.projectIds.includes(check.projectId)) {
      fail('invalid_check', 'Check must belong to a selected project');
    }
  }
  for (const a of c.acceptance) {
    if (a.checkIds?.some((i) => !c.checkIds.includes(i))) {
      fail('undeclared_check', 'Acceptance check is not declared');
    }
  }
  return c;
}
export async function createTask(
  workspace: ResolvedWorkspace,
  store: SpaceStore,
  input: TaskInput,
): Promise<TaskState> {
  store.assertSpace(workspace);
  const c = validateContract(workspace, input);
  return store.withLock(async () => {
    if (await maybeJson(path.join(store.root, 'tasks', c.id, 'state.json'))) {
      fail('task_exists', 'Task already exists', 5);
    }
    await atomicJson(
      path.join(workspace.workspaceDir, 'tasks', c.id, 'contracts', '1.json'),
      { ...c, revision: 1 },
      true,
    );
    const state: TaskState = {
      schemaVersion: 1,
      workspaceId: c.workspaceId,
      id: c.id,
      revision: 1,
      phase: 'draft',
      condition: 'clear',
      contractRevision: 1,
      runIds: [],
      issues: [],
    };
    await store.writeTask(state, 0);
    return state;
  });
}
export async function reviseTask(
  workspace: ResolvedWorkspace,
  store: SpaceStore,
  taskId: string,
  input: TaskRevisionInput,
): Promise<TaskState> {
  store.assertSpace(workspace);
  if (!input.reason?.trim()) {
    fail('missing_reason', 'Contract revision requires a reason');
  }
  const c = validateContract(workspace, input.contract);
  if (c.id !== taskId) {
    fail('task_mismatch', 'Revision task ID differs');
  }
  return store.withLock(async () => {
    const state = await store.readTask(taskId);
    if (state.contractRevision !== input.expectedRevision) {
      fail('revision_conflict', 'Contract revision changed', 5);
    }
    if (['complete', 'cancelled'].includes(state.phase)) {
      fail('terminal_task', 'Cannot revise terminal task', 3);
    }
    const revision = state.contractRevision + 1;
    await atomicJson(
      path.join(
        workspace.workspaceDir,
        'tasks',
        taskId,
        'contracts',
        `${revision}.json`,
      ),
      {
        ...c,
        revision,
        previousRevision: state.contractRevision,
        reason: input.reason,
        decisionRef: input.decisionRef,
      },
      true,
    );
    const next: TaskState = {
      ...state,
      revision: state.revision + 1,
      phase: 'draft',
      condition: state.snapshotId ? 'needs_rebaseline' : 'clear',
      contractRevision: revision,
      issues: [],
    };
    await store.writeTask(next, state.revision);
    return next;
  });
}
export async function verifyUnlocked(
  workspace: ResolvedWorkspace,
  store: SpaceStore,
  taskId: string,
): Promise<VerificationResult> {
  const state = await store.readTask(taskId);
  const contract = await store.readContract(taskId, state.contractRevision);
  const issues: Issue[] = [];
  const reusableRunIds: string[] = [];
  const staleRunIds: string[] = [];
  let drift = false;
  let integrityFailed = false;
  if (!state.snapshotId) {
    issues.push({
      code: 'missing_snapshot',
      severity: 'error',
      message: 'Task has no prepared snapshot',
    });
  } else {
    let manifest;
    try {
      manifest = await verifySnapshotObjects(
        store,
        taskId,
        state.snapshotId,
        state.originId,
      );
    } catch {
      integrityFailed = true;
      issues.push({
        code: 'snapshot_incomplete',
        severity: 'error',
        message:
          'Snapshot or original differences are missing, damaged or mismatched',
      });
    }
    if (manifest) {
      const basis = await contextDrift(workspace, store, contract, manifest);
      issues.push(...basis);
      drift = basis.length > 0;
    }
  }
  const current = await candidates(workspace, contract);
  const validChecks = new Set<string>();
  let blocked = integrityFailed;
  let failed = false;
  for (const checkId of contract.checkIds) {
    const check = workspace.definition.checks.find((c) => c.id === checkId);
    if (!check) {
      issues.push({
        code: 'missing_check',
        severity: 'error',
        message: 'Declared check removed',
      });
      blocked = true;
      continue;
    }
    const isRequired =
      check.required ||
      contract.acceptance.some(
        (a) => a.required !== false && a.checkIds?.includes(checkId),
      );
    const proof = await probe(workspace, check, store);
    if (proof.issues.length) {
      issues.push(
        ...proof.issues.map((i) => ({
          ...i,
          severity: isRequired ? ('error' as const) : ('warning' as const),
        })),
      );
      blocked ||= isRequired;
    }
    let matching = false;
    for (const runId of [...state.runIds].reverse()) {
      let run;
      try {
        run = await store.readRun(runId);
      } catch {
        issues.push({
          code: 'incomplete_run',
          severity: 'warning',
          message: 'Incomplete run cannot be reused',
        });
        continue;
      }
      if (run.checkId !== checkId) {
        continue;
      }
      const matches =
        run.sealed === true &&
        run.evidenceSource === 'cli-captured' &&
        run.workspaceId === workspace.definition.id &&
        run.taskId === taskId &&
        run.contractRevision === contract.revision &&
        run.snapshotId === state.snapshotId &&
        run.checkHash === digest(check) &&
        run.candidateBefore.hash === current[check.projectId]?.hash &&
        run.candidateAfter.hash === current[check.projectId]?.hash &&
        run.bindingRevision === current[check.projectId]?.bindingRevision &&
        digest(run.tools) === digest(proof.tools) &&
        digest(run.environment) === digest(proof.environment) &&
        !proof.issues.length &&
        !drift &&
        !integrityFailed;
      let intact = true;
      try {
        const runDir = path.join(store.root, 'runs', run.runId);
        if (
          hash(await readFile(path.join(runDir, 'stdout.log'))) !==
            run.stdoutHash ||
          hash(await readFile(path.join(runDir, 'stderr.log'))) !==
            run.stderrHash
        ) {
          intact = false;
        }
        for (const a of run.artifacts) {
          if (
            hash(await readFile(await contained(runDir, a.path))) !== a.hash
          ) {
            intact = false;
          }
        }
      } catch {
        intact = false;
      }
      if (!matches || !intact) {
        staleRunIds.push(runId);
        continue;
      }
      if (!matching) {
        matching = true;
        if (run.result === 'passed') {
          validChecks.add(checkId);
          reusableRunIds.push(runId);
        } else if (check.required) {
          failed = run.result === 'failed';
          blocked ||= run.result === 'blocked';
        }
      }
    }
    if (check.required && !validChecks.has(checkId)) {
      issues.push({
        code: 'required_check_unsatisfied',
        severity: 'error',
        message: `Required check has no current passed evidence: ${checkId}`,
      });
    }
  }
  const acceptance = [];
  for (const a of contract.acceptance) {
    let satisfied = false;
    if (a.kind === 'manual') {
      blocked ||= a.required !== false;
      issues.push({
        code: 'unsupported_manual_acceptance',
        severity: a.required === false ? 'warning' : 'error',
        message: 'Manual acceptance writing is unsupported',
      });
    } else if (a.kind === 'artifact' && a.artifact) {
      const p = workspace.definition.projects.find(
        (p) => p.id === a.artifact!.projectId,
      );
      try {
        if (!p) {
          throw Error();
        }
        await contained(workspace.bindings[p.binding]!, a.artifact.path);
        satisfied = true;
      } catch {
        /* Missing artifact remains unsatisfied. */
      }
    } else {
      satisfied =
        !!a.checkIds?.length && a.checkIds.every((id) => validChecks.has(id));
    }
    acceptance.push({ id: a.id, satisfied });
    if (a.required !== false && !satisfied) {
      issues.push({
        code: 'acceptance_unsatisfied',
        severity: 'error',
        message: `Acceptance item not satisfied: ${a.id}`,
      });
    }
  }
  let phase: VerificationResult['phase'] = state.phase;
  let condition: VerificationResult['condition'] = 'clear';
  if (drift) {
    condition = 'needs_rebaseline';
  } else if (blocked || !state.snapshotId) {
    condition = 'blocked';
  } else if (issues.some((i) => i.severity === 'error')) {
    condition = 'failed';
  }
  if (state.phase !== 'complete') {
    if (condition === 'clear') {
      phase = 'verified';
    } else if (state.snapshotId) {
      phase = state.runIds.length ? 'verifying' : 'ready';
    }
  }
  if (failed && condition === 'clear') {
    condition = 'failed';
  }
  const result: VerificationResult = {
    phase,
    condition,
    issues,
    acceptance,
    reusableRunIds,
    staleRunIds,
  };
  await store.writeTask(
    {
      ...state,
      revision: state.revision + 1,
      phase,
      condition,
      issues,
    },
    state.revision,
  );
  return result;
}
export async function verifyTask(
  workspace: ResolvedWorkspace,
  store: SpaceStore,
  taskId: string,
): Promise<VerificationResult> {
  store.assertSpace(workspace);
  return store.withLock(() => verifyUnlocked(workspace, store, taskId));
}
const notesSchema = z.strictObject({
  schemaVersion: z.literal(1),
  taskId: id,
  contractRevision: z.number().int().min(1),
  snapshotId: z.string().regex(/^snap_[a-f0-9]{64}$/),
  completedClaims: z.array(
    z.strictObject({
      text: z.string().min(1),
      evidenceRunIds: z.array(z.string()).optional(),
    }),
  ),
  openIssues: z.array(z.string()),
  nextSteps: z.array(z.string()),
  externalActions: z.array(
    z.strictObject({
      description: z.string().min(1),
      status: z.enum(['observed', 'unknown']),
      evidenceRunIds: z.array(z.string()).optional(),
    }),
  ),
});
export async function createHandoff(
  workspace: ResolvedWorkspace,
  store: SpaceStore,
  taskId: string,
  notes: HandoffNotes,
): Promise<Checkpoint> {
  store.assertSpace(workspace);
  const parsed = notesSchema.safeParse(notes);
  if (!parsed.success) {
    fail('invalid_handoff', parsed.error.message);
  }
  return store.withLock(async () => {
    let state = await store.readTask(taskId);
    if (
      notes.taskId !== taskId ||
      notes.contractRevision !== state.contractRevision ||
      notes.snapshotId !== state.snapshotId
    ) {
      fail(
        'handoff_mismatch',
        'Handoff does not match current task contract and snapshot',
        4,
      );
    }
    const verification = await verifyUnlocked(workspace, store, taskId);
    state = await store.readTask(taskId);
    for (const claim of [...notes.completedClaims, ...notes.externalActions]) {
      for (const ref of claim.evidenceRunIds ?? []) {
        if (!verification.reusableRunIds.includes(ref)) {
          fail(
            'handoff_evidence_mismatch',
            'Handoff cites evidence that is not currently reusable',
            4,
          );
        }
      }
    }
    const contract = await store.readContract(taskId, state.contractRevision);
    const id = 'cp_' + randomUUID();
    const summary = `# ${contract.goal}\n\nTask: ${taskId}; contract: ${state.contractRevision}; snapshot: ${state.snapshotId}\n\nPhase: ${verification.phase}; condition: ${verification.condition}\n\n## Completed claims\n${notes.completedClaims.map((c) => `- ${c.text} [${c.evidenceRunIds?.length ? 'current run evidence: ' + c.evidenceRunIds.join(', ') : 'unproven statement'}]`).join('\n')}\n\n## Open issues\n${notes.openIssues.map((s) => '- ' + s).join('\n')}\n\n## Next steps\n${notes.nextSteps.map((s) => '- ' + s).join('\n')}\n\n## External actions\n${notes.externalActions.map((a) => `- ${a.description}: ${a.status}`).join('\n')}\n`;
    const checkpoint: Checkpoint = {
      schemaVersion: 1,
      id,
      workspaceId: workspace.definition.id,
      taskId,
      contractRevision: state.contractRevision,
      snapshotId: state.snapshotId!,
      goal: contract.goal,
      scope: contract.scope,
      originId: state.originId!,
      candidate: await candidates(workspace, contract),
      verification,
      notes,
      createdAt: new Date().toISOString(),
      summary,
    };
    await atomicJson(
      path.join(store.root, 'checkpoints', id, 'checkpoint.json'),
      checkpoint,
      true,
    );
    await writeFile(
      path.join(store.root, 'checkpoints', id, 'summary.md'),
      summary,
      { flag: 'wx', mode: 0o600 },
    );
    await store.writeTask(
      {
        ...state,
        revision: state.revision + 1,
        checkpointId: id,
        phase: state.phase === 'ready' ? 'active' : state.phase,
      },
      state.revision,
    );
    return checkpoint;
  });
}
export async function resumeTask(
  workspace: ResolvedWorkspace,
  store: SpaceStore,
  taskId: string,
): Promise<ResumeResult> {
  store.assertSpace(workspace);
  return store.withLock(async () => {
    const verification = await verifyUnlocked(workspace, store, taskId);
    const state = await store.readTask(taskId);
    const differences = [...verification.issues];
    let nextSteps: string[] = [];
    if (state.checkpointId) {
      const checkpoint = await store.readCheckpoint(state.checkpointId);
      if (
        checkpoint.contractRevision !== state.contractRevision ||
        checkpoint.snapshotId !== state.snapshotId
      ) {
        differences.push({
          code: 'checkpoint_stale',
          severity: 'warning',
          message:
            'Latest checkpoint describes a previous task contract or snapshot',
        });
      }
      const contract = await store.readContract(taskId, state.contractRevision);
      if (
        digest(checkpoint.candidate) !==
        digest(await candidates(workspace, contract))
      ) {
        differences.push({
          code: 'candidate_changed',
          severity: 'warning',
          message: 'Engineering candidate differs from checkpoint',
        });
      }
      nextSteps = checkpoint.notes.nextSteps;
      for (const action of checkpoint.notes.externalActions) {
        if (action.status === 'unknown') {
          nextSteps = [
            `Inspect actual external action before retrying: ${action.description}`,
            ...nextSteps,
          ];
        }
      }
    } else {
      differences.push({
        code: 'missing_checkpoint',
        severity: 'warning',
        message: 'No handoff checkpoint exists',
      });
    }
    return {
      checkpointId: state.checkpointId,
      differences,
      reusableRunIds: verification.reusableRunIds,
      nextSteps,
      state,
    };
  });
}
export async function completeTask(
  workspace: ResolvedWorkspace,
  store: SpaceStore,
  taskId: string,
): Promise<TaskState> {
  store.assertSpace(workspace);
  return store.withLock(async () => {
    const verification = await verifyUnlocked(workspace, store, taskId);
    const state = await store.readTask(taskId);
    if (
      verification.condition !== 'clear' ||
      !['verified', 'complete'].includes(verification.phase)
    ) {
      fail(
        'not_verified',
        'Current task evidence does not satisfy acceptance',
        verification.condition === 'needs_rebaseline' ? 4 : 3,
      );
    }
    if (!state.checkpointId) {
      fail('missing_handoff', 'Completion requires current handoff', 3);
    }
    const contract = await store.readContract(taskId, state.contractRevision);
    if (contract.delivery.requireHumanAcceptance) {
      fail(
        'unsupported_manual_acceptance',
        'Required human acceptance unsupported in first loop',
        3,
      );
    }
    const checkpoint = await store.readCheckpoint(state.checkpointId);
    if (
      checkpoint.contractRevision !== state.contractRevision ||
      checkpoint.snapshotId !== state.snapshotId ||
      digest(checkpoint.candidate) !==
        digest(await candidates(workspace, contract)) ||
      digest(checkpoint.verification.reusableRunIds) !==
        digest(verification.reusableRunIds)
    ) {
      fail(
        'stale_handoff',
        'Handoff does not match current delivery candidate and evidence',
        4,
      );
    }
    for (const artifact of contract.delivery.artifacts) {
      const project = workspace.definition.projects.find(
        (p) => p.id === artifact.projectId,
      );
      if (!project) {
        fail('missing_delivery', 'Delivery project missing', 3);
      }
      try {
        await contained(workspace.bindings[project.binding]!, artifact.path);
      } catch {
        fail('missing_delivery', 'Delivery artifact is missing or unsafe', 3);
      }
    }
    const result: TaskState = {
      ...state,
      revision: state.revision + 1,
      phase: 'complete',
      condition: 'clear',
    };
    await store.writeTask(result, state.revision);
    return result;
  });
}
