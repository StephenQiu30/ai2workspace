import type { KnowledgeDocument } from '../knowledge/types.js';
import { selectContext } from '../context/select.js';
import { readFile } from 'node:fs/promises';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import path from 'node:path';
import type { ResolvedWorkspace } from '../workspace/types.js';
import type {
  Fingerprint,
  TaskContract,
  SnapshotManifest,
  TaskOrigin,
  PrepareOptions,
  PrepareResult,
  ContextBundle,
} from '../harness/types.js';
import type { Issue } from '../shared/types.js';
import {
  SpaceStore,
  putObject,
  readObject,
  hash,
  digest,
  contained,
  atomicJson,
  readJson,
  maybeJson,
  fail,
} from '../storage/file-store.js';
import {
  scanUnlocked,
  documentKey,
  listFiles,
  excluded,
} from '../knowledge/scan.js';
import { loadHarnessPolicy, contractPolicyIssues } from './policy.js';
const exec = promisify(execFile);
export async function fingerprint(
  workspace: ResolvedWorkspace,
  projectId: string,
): Promise<Fingerprint> {
  const project = workspace.definition.projects.find((p) => p.id === projectId);
  if (!project) {
    fail('unknown_project', 'Unknown project');
  }
  const root = workspace.bindings[project.binding];
  if (!root) {
    fail('missing_binding', 'Project binding unavailable', 3);
  }
  const files = [];
  for (const rel of await listFiles(root, project.inputExclude, [], true)) {
    if (
      path.resolve(root, rel).startsWith(workspace.dataDir + path.sep) ||
      path
        .resolve(root, rel)
        .startsWith(path.join(workspace.workspaceDir, 'tasks') + path.sep) ||
      path.resolve(root, rel) ===
        path.join(workspace.workspaceDir, 'catalog.json')
    ) {
      continue;
    }
    files.push({
      path: rel,
      hash: hash(await readFile(await contained(root, rel))),
    });
  }
  let head: string | undefined;
  try {
    head = (
      await exec('git', ['-C', root, 'rev-parse', 'HEAD'], {
        timeout: 5000,
        maxBuffer: 1024 * 1024,
      })
    ).stdout.trim();
  } catch {
    /* Ordinary directories have no Git HEAD. */
  }
  // The selected project definition owns the input boundary as well as its binding.
  const bindingRevision = digest({ project, path: root });
  return {
    hash: digest({ files, head, bindingRevision }),
    files,
    head,
    bindingRevision,
  };
}
export async function candidates(
  workspace: ResolvedWorkspace,
  contract: TaskContract,
): Promise<Record<string, Fingerprint>> {
  const result: Record<string, Fingerprint> = {};
  for (const id of contract.projectIds) {
    result[id] = await fingerprint(workspace, id);
  }
  return result;
}
export async function readManifest(
  store: SpaceStore,
  id: string,
): Promise<SnapshotManifest> {
  if (!/^snap_[a-f0-9]{64}$/.test(id)) {
    fail('invalid_snapshot', 'Invalid snapshot ID');
  }
  const m = await readJson<SnapshotManifest>(
    path.join(store.root, 'snapshots', id, 'manifest.json'),
  );
  if ('snap_' + digest(m) !== id) {
    fail('corrupt_snapshot', 'Snapshot hash mismatch', 3);
  }
  return m;
}
function outputDocument(
  contract: TaskContract,
  doc: KnowledgeDocument,
  workspace: ResolvedWorkspace,
): boolean {
  const source = workspace.definition.sources.find(
    (s) => s.id === doc.sourceId,
  )!;
  if (source.kind === 'instruction') {
    return false;
  }
  return contract.scope.outputs.some((o) =>
    'sourceId' in o
      ? o.sourceId === doc.sourceId && o.documentId === doc.documentId
      : workspace.definition.projects.some(
          (p) =>
            p.id === o.projectId &&
            p.binding === source.binding &&
            o.path === doc.relativePath,
        ),
  );
}
function governanceHash(doc: KnowledgeDocument): string {
  return digest({
    type: doc.metadata.type,
    domain: doc.metadata.domain,
    projectIds: doc.metadata.projectIds,
    source: doc.metadata.source,
    dependencies: doc.metadata.dependencies,
  });
}
export async function contextDrift(
  workspace: ResolvedWorkspace,
  store: SpaceStore,
  contract: TaskContract,
  manifest: SnapshotManifest,
): Promise<Issue[]> {
  const issues: Issue[] = [];
  const policy = await loadHarnessPolicy(workspace);
  issues.push(...contractPolicyIssues(workspace, contract, policy));
  if (manifest.policyHash !== policy.hash) {
    issues.push({
      code: 'harness_policy_changed',
      severity: 'error',
      message:
        'Harness policy or role ownership changed; explicit baseline review required',
    });
  }
  if (
    manifest.contractHash !== digest(contract) ||
    manifest.profileHash !==
      digest({
        schemaVersion: 1,
        profile: workspace.definition.profile,
        sources: workspace.definition.sources,
      })
  ) {
    issues.push({
      code: 'contract_changed',
      severity: 'error',
      message: 'Contract or governance profile changed',
    });
  }
  for (const [sourceId, bindingHash] of Object.entries(
    manifest.sourceBindings,
  )) {
    const s = workspace.definition.sources.find((s) => s.id === sourceId);
    if (!s || digest(workspace.bindings[s.binding] ?? null) !== bindingHash) {
      issues.push({
        code: 'binding_changed',
        severity: 'error',
        message: 'Knowledge binding changed',
      });
    }
  }
  const scan = await scanUnlocked(workspace, store);
  for (const before of manifest.documents) {
    const now = scan.documents.find(
      (d) => documentKey(d) === documentKey(before),
    );
    if (
      !now ||
      (!outputDocument(contract, now, workspace) &&
        (now.hash !== before.hash ||
          digest(now.metadata) !== before.metadataHash)) ||
      (outputDocument(contract, now, workspace) &&
        governanceHash(now) !== before.governanceHash)
    ) {
      issues.push({
        code: 'basis_changed',
        severity: 'error',
        message: 'Selected basis changed or unavailable',
        documentId: before.documentId,
      });
    }
  }
  const selected = new Set(manifest.documents.map(documentKey));
  for (const d of scan.documents) {
    const s = workspace.definition.sources.find((s) => s.id === d.sourceId)!;
    if (
      s.kind === 'instruction' &&
      (!s.projectId || contract.projectIds.includes(s.projectId)) &&
      !selected.has(documentKey(d))
    ) {
      issues.push({
        code: 'rules_changed',
        severity: 'error',
        message: 'New applicable instruction requires baseline review',
        documentId: d.documentId,
      });
    }
  }
  for (const a of manifest.attachments) {
    const source = workspace.definition.sources.find(
      (s) => s.id === a.sourceId,
    );
    const root = source && workspace.bindings[source.binding];
    try {
      if (
        !root ||
        hash(await readFile(await contained(root, a.relativePath))) !== a.hash
      ) {
        throw Error('changed');
      }
    } catch {
      issues.push({
        code: 'attachment_changed',
        severity: 'error',
        message: 'Selected attachment changed or unavailable',
        path: a.relativePath,
      });
    }
  }
  for (const id of contract.projectIds) {
    const current = await fingerprint(workspace, id);
    if (
      current.bindingRevision !== manifest.projectBaselines[id]?.bindingRevision
    ) {
      issues.push({
        code: 'binding_changed',
        severity: 'error',
        message: 'Engineering binding or input policy changed',
      });
    }
  }
  return issues;
}
export async function prepareTask(
  workspace: ResolvedWorkspace,
  store: SpaceStore,
  taskId: string,
  options: PrepareOptions = {},
): Promise<PrepareResult> {
  store.assertSpace(workspace);
  if (options.rebaseline && !options.reason?.trim()) {
    fail('missing_reason', 'Rebaseline requires a reason');
  }
  return store.withLock(async () => {
    const state = await store.readTask(taskId);
    if (['complete', 'cancelled'].includes(state.phase)) {
      fail('terminal_task', 'Terminal task cannot be prepared', 3);
    }
    const contract = await store.readContract(taskId, state.contractRevision);
    const policy = await loadHarnessPolicy(workspace);
    const policyIssues = contractPolicyIssues(workspace, contract, policy);
    if (policyIssues.length) {
      await store.writeTask(
        {
          ...state,
          revision: state.revision + 1,
          condition: 'blocked',
          issues: policyIssues,
        },
        state.revision,
      );
      return { ready: false, issues: policyIssues };
    }
    if (
      state.snapshotId &&
      !policy.hash &&
      (await readManifest(store, state.snapshotId)).policyHash
    ) {
      const issues: Issue[] = [
        {
          code: 'harness_policy_removed',
          severity: 'error',
          message:
            'A prepared policy task cannot drop its Harness policy during rebaseline',
        },
      ];
      await store.writeTask(
        {
          ...state,
          revision: state.revision + 1,
          condition: 'blocked',
          issues,
        },
        state.revision,
      );
      return { ready: false, snapshotId: state.snapshotId, issues };
    }
    if (state.snapshotId && !options.rebaseline) {
      const issues = await contextDrift(
        workspace,
        store,
        contract,
        await readManifest(store, state.snapshotId),
      );
      if (issues.length) {
        await store.writeTask(
          {
            ...state,
            revision: state.revision + 1,
            condition: 'needs_rebaseline',
            issues,
          },
          state.revision,
        );
        return { ready: false, snapshotId: state.snapshotId, issues };
      }
      return { ready: true, snapshotId: state.snapshotId, issues: [] };
    }
    const scan = await scanUnlocked(workspace, store);
    const selection = selectContext(scan, {
      instructions: scan.documents.filter((d) => {
        const source = workspace.definition.sources.find(
          (s) => s.id === d.sourceId,
        )!;
        return (
          source.kind === 'instruction' &&
          (!source.projectId || contract.projectIds.includes(source.projectId))
        );
      }),
      inputs: contract.inputs,
    });
    const issues: Issue[] = [...workspace.issues, ...selection.issues];
    const missingOptional = selection.missingOptional;
    const selected = selection.documents;
    if (issues.some((i) => i.severity === 'error')) {
      await store.writeTask(
        {
          ...state,
          revision: state.revision + 1,
          condition: 'blocked',
          issues,
        },
        state.revision,
      );
      return { ready: false, issues };
    }
    const projectBaselines = await candidates(workspace, contract);
    let originId =
      state.originId ??
      (
        await maybeJson<TaskOrigin>(
          path.join(store.root, 'tasks', taskId, 'origin.json'),
        )
      )?.id;
    if (!originId) {
      const initialFiles: TaskOrigin['initialFiles'] = [];
      for (const [projectId, baseline] of Object.entries(projectBaselines)) {
        const project = workspace.definition.projects.find(
          (p) => p.id === projectId,
        )!;
        const root = workspace.bindings[project.binding]!;
        let initialPaths = new Set(baseline.files.map((f) => f.path));
        if (baseline.head) {
          try {
            const modified = (
              await exec(
                'git',
                ['-C', root, 'diff', '--name-only', '-z', 'HEAD'],
                { maxBuffer: 16 * 1024 * 1024 },
              )
            ).stdout.split('\0');
            const untracked = (
              await exec(
                'git',
                [
                  '-C',
                  root,
                  'ls-files',
                  '--others',
                  '--exclude-standard',
                  '-z',
                ],
                { maxBuffer: 16 * 1024 * 1024 },
              )
            ).stdout.split('\0');
            initialPaths = new Set([...modified, ...untracked]);
          } catch {
            issues.push({
              code: 'origin_diff_unavailable',
              severity: 'warning',
              message:
                'Git initial difference unavailable; allowed file copies retained',
            });
          }
        }
        for (const f of baseline.files) {
          if (initialPaths.has(f.path)) {
            initialFiles.push({
              projectId,
              path: f.path,
              hash: f.hash,
              objectPath: await putObject(
                store,
                await readFile(await contained(root, f.path)),
              ),
            });
          }
        }
      }
      originId = 'origin_' + digest({ projectBaselines, initialFiles });
      const origin: TaskOrigin = {
        schemaVersion: 1,
        id: originId,
        workspaceId: workspace.definition.id,
        taskId,
        createdAt: new Date().toISOString(),
        limitation: options.latePrepare
          ? 'late_prepare'
          : 'First preparation captures observed differences; authorship is unknown',
        projects: projectBaselines,
        initialFiles,
      };
      await atomicJson(
        path.join(store.root, 'tasks', taskId, 'origin.json'),
        origin,
        true,
      );
    }
    const documents: SnapshotManifest['documents'] = [];
    const attachments: SnapshotManifest['attachments'] = [];
    for (const { doc, purpose, section } of selected.values()) {
      const source = workspace.definition.sources.find(
        (s) => s.id === doc.sourceId,
      )!;
      const root = workspace.bindings[source.binding]!;
      documents.push({
        workspaceId: doc.workspaceId,
        sourceId: doc.sourceId,
        documentId: doc.documentId,
        relativePath: doc.relativePath,
        purpose,
        status: doc.metadata.status,
        hash: doc.hash,
        objectPath: await putObject(store, doc.body),
        section,
        metadataHash: digest(doc.metadata),
        governanceHash: governanceHash(doc),
        kind: source.kind,
      });
      for (const link of doc.links) {
        if (/^(?:[a-z]+:|#)/i.test(link)) {
          continue;
        }
        const target = decodeURIComponent(link.split('#')[0]!);
        if (!target || target.endsWith('.md')) {
          continue;
        }
        const rel = path.posix.normalize(
          path.posix.join(path.posix.dirname(doc.relativePath), target),
        );
        if (
          !/\.(?:png|jpe?g|gif|webp|avif|pdf|txt|csv|json)$/i.test(rel) ||
          excluded(rel, source.exclude)
        ) {
          issues.push({
            code: 'unsupported_attachment',
            severity: 'warning',
            message: 'Attachment not included',
            path: rel,
          });
          continue;
        }
        try {
          const bytes = await readFile(await contained(root, rel));
          attachments.push({
            sourceId: source.id,
            relativePath: rel,
            hash: hash(bytes),
            objectPath: await putObject(store, bytes),
          });
        } catch {
          issues.push({
            code: 'missing_attachment',
            severity: 'error',
            message: 'Selected attachment unavailable',
            path: rel,
          });
        }
      }
    }
    for (const { doc } of selected.values()) {
      const source = workspace.definition.sources.find(
        (s) => s.id === doc.sourceId,
      )!;
      try {
        if (
          hash(
            await readFile(
              await contained(
                workspace.bindings[source.binding]!,
                doc.relativePath,
              ),
            ),
          ) !== doc.hash
        ) {
          throw Error('changed');
        }
      } catch {
        issues.push({
          code: 'prepare_input_changed',
          severity: 'error',
          message: 'Input changed during preparation',
          documentId: doc.documentId,
        });
      }
    }
    for (const a of attachments) {
      const s = workspace.definition.sources.find((s) => s.id === a.sourceId)!;
      if (
        hash(
          await readFile(
            await contained(workspace.bindings[s.binding]!, a.relativePath),
          ),
        ) !== a.hash
      ) {
        issues.push({
          code: 'prepare_input_changed',
          severity: 'error',
          message: 'Attachment changed during preparation',
          path: a.relativePath,
        });
      }
    }
    if (
      digest(await candidates(workspace, contract)) !== digest(projectBaselines)
    ) {
      issues.push({
        code: 'prepare_input_changed',
        severity: 'error',
        message: 'Project changed during preparation',
      });
    }
    if (issues.some((i) => i.severity === 'error')) {
      await store.writeTask(
        {
          ...state,
          originId,
          revision: state.revision + 1,
          condition: 'needs_rebaseline',
          issues,
        },
        state.revision,
      );
      return { ready: false, issues };
    }
    const sourceRevisions: Record<string, string> = {};
    const sourceBindings: Record<string, string> = {};
    for (const doc of documents) {
      sourceRevisions[doc.sourceId] = digest(
        documents.filter((d) => d.sourceId === doc.sourceId),
      );
      const s = workspace.definition.sources.find(
        (s) => s.id === doc.sourceId,
      )!;
      sourceBindings[doc.sourceId] = digest(
        workspace.bindings[s.binding] ?? null,
      );
    }
    const manifest: SnapshotManifest = {
      schemaVersion: 1,
      workspaceId: workspace.definition.id,
      contractHash: digest(contract),
      profileHash: digest({
        schemaVersion: 1,
        profile: workspace.definition.profile,
        sources: workspace.definition.sources,
      }),
      sourceRevisions,
      sourceBindings,
      documents,
      attachments,
      projectBaselines,
      missingOptional,
      originId,
      ...(policy.hash ? { policyHash: policy.hash } : {}),
      ...(policy.hash && policy.policy && policy.roles && policy.projectId
        ? {
            policySnapshot: {
              hash: policy.hash,
              projectId: policy.projectId,
              policy: policy.policy,
              roles: policy.roles,
            },
          }
        : {}),
    };
    const snapshotId = 'snap_' + digest(manifest);
    await atomicJson(
      path.join(store.root, 'snapshots', snapshotId, 'manifest.json'),
      manifest,
      true,
    );
    if (state.snapshotId && options.rebaseline) {
      await atomicJson(
        path.join(
          store.root,
          'tasks',
          taskId,
          'rebaselines',
          `${state.revision + 1}.json`,
        ),
        {
          previous: state.snapshotId,
          current: snapshotId,
          reason: options.reason,
        },
        true,
      );
    }
    await store.writeTask(
      {
        ...state,
        revision: state.revision + 1,
        phase: 'ready',
        condition: 'clear',
        originId,
        snapshotId,
        issues,
      },
      state.revision,
    );
    return { snapshotId, ready: true, issues };
  });
}
export async function exportContext(
  workspace: ResolvedWorkspace,
  store: SpaceStore,
  taskId: string,
): Promise<ContextBundle> {
  store.assertSpace(workspace);
  const state = await store.readTask(taskId);
  if (!state.snapshotId) {
    fail('missing_snapshot', 'Prepare task first', 3);
  }
  const manifest = await readManifest(store, state.snapshotId);
  const documents = [];
  for (const doc of manifest.documents) {
    documents.push({
      ...doc,
      body: (await readObject(store, doc.objectPath, doc.hash)).toString(
        'utf8',
      ),
    });
  }
  return {
    snapshotId: state.snapshotId,
    manifest,
    documents,
    attachments: manifest.attachments,
    groups: {
      rules: documents.filter((d) => d.kind === 'instruction').map(documentKey),
      confirmed: documents
        .filter((d) => d.status === 'confirmed')
        .map(documentKey),
      candidates: documents
        .filter((d) => d.kind !== 'instruction' && d.status !== 'confirmed')
        .map(documentKey),
    },
    gaps: manifest.missingOptional,
    ...(manifest.policySnapshot
      ? { policySnapshot: manifest.policySnapshot }
      : {}),
  };
}

export async function verifySnapshotObjects(
  store: SpaceStore,
  taskId: string,
  snapshotId: string,
  originId: string | undefined,
): Promise<SnapshotManifest> {
  const manifest = await readManifest(store, snapshotId);
  if (
    manifest.workspaceId !== store.workspace.definition.id ||
    manifest.originId !== originId
  ) {
    fail('snapshot_mismatch', 'Snapshot origin or workspace mismatch', 3);
  }
  for (const doc of manifest.documents) {
    if (doc.workspaceId !== manifest.workspaceId) {
      fail(
        'snapshot_mismatch',
        'Snapshot document belongs to another space',
        3,
      );
    }
    await readObject(store, doc.objectPath, doc.hash);
  }
  for (const attachment of manifest.attachments) {
    await readObject(store, attachment.objectPath, attachment.hash);
  }
  const origin = await readJson<TaskOrigin>(
    path.join(store.root, 'tasks', taskId, 'origin.json'),
  );
  if (
    origin.id !== originId ||
    origin.workspaceId !== manifest.workspaceId ||
    origin.taskId !== taskId ||
    origin.id !==
      'origin_' +
        digest({
          projectBaselines: origin.projects,
          initialFiles: origin.initialFiles,
        })
  ) {
    fail('corrupt_origin', 'Task origin identity mismatch', 3);
  }
  for (const initial of origin.initialFiles) {
    await readObject(store, initial.objectPath, initial.hash);
  }
  return manifest;
}
