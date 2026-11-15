import { createHash, randomUUID } from 'node:crypto';
import {
  mkdir,
  readFile,
  rename,
  open,
  unlink,
  realpath,
  lstat,
  readdir,
  link,
} from 'node:fs/promises';
import path from 'node:path';
import { hostname } from 'node:os';
import type { Catalog, ScanResult } from '../knowledge/types.js';
import type {
  TaskState,
  TaskContract,
  RunRecord,
  Checkpoint,
} from '../harness/types.js';
import type { ResolvedWorkspace } from '../workspace/types.js';
import type { Issue } from '../shared/types.js';
export class CoreError extends Error {
  constructor(
    public exitCode: number,
    public issues: Issue[],
  ) {
    super(issues.map((i) => i.message).join('; '));
    this.name = 'CoreError';
  }
}
export function fail(code: string, message: string, exitCode = 2): never {
  throw new CoreError(exitCode, [{ code, severity: 'error', message }]);
}
export function canonical(value: unknown): string {
  if (Array.isArray(value)) {
    return '[' + value.map(canonical).join(',') + ']';
  }
  if (value !== null && typeof value === 'object') {
    return (
      '{' +
      Object.entries(value)
        .filter(([, v]) => v !== undefined)
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([k, v]) => JSON.stringify(k) + ':' + canonical(v))
        .join(',') +
      '}'
    );
  }
  return JSON.stringify(value);
}
export function hash(value: string | Buffer): string {
  return createHash('sha256').update(value).digest('hex');
}
export function digest(value: unknown): string {
  return hash(canonical(value));
}
export function safeId(id: string): string {
  if (!/^[a-z0-9][a-z0-9-]{0,63}$/.test(id)) {
    fail('invalid_id', `Invalid logical ID: ${id}`);
  }
  return id;
}
export function relativePath(value: string): string {
  if (
    !value ||
    path.isAbsolute(value) ||
    value.includes('\\') ||
    value.split('/').some((x) => x === '..') ||
    value.includes('\0')
  ) {
    fail('path_outside_root', 'Expected relative contained path');
  }
  return value;
}
export async function contained(
  root: string,
  relative: string,
): Promise<string> {
  relativePath(relative);
  const base = await realpath(root);
  const target = path.resolve(base, relative);
  if (target !== base && !target.startsWith(base + path.sep)) {
    fail('path_outside_root', 'Path escapes binding');
  }
  let cursor = base;
  for (const part of path
    .relative(base, target)
    .split(path.sep)
    .filter(Boolean)) {
    cursor = path.join(cursor, part);
    const s = await lstat(cursor);
    if (s.isSymbolicLink()) {
      fail('symlink_excluded', 'Symbolic links are not followed', 3);
    }
  }
  return target;
}
export async function readJson<T>(file: string): Promise<T> {
  return JSON.parse(await readFile(file, 'utf8')) as T;
}
export async function maybeJson<T>(file: string): Promise<T | undefined> {
  try {
    return await readJson<T>(file);
  } catch (e) {
    if ((e as NodeJS.ErrnoException).code === 'ENOENT') {
      return;
    }
    throw e;
  }
}
export async function atomicJson(
  file: string,
  value: unknown,
  immutable = false,
): Promise<void> {
  await mkdir(path.dirname(file), { recursive: true });
  const bytes = canonical(value) + '\n';
  JSON.parse(bytes);
  const tmp = file + '.tmp-' + randomUUID();
  try {
    const fd = await open(tmp, 'wx', 0o600);
    try {
      await fd.writeFile(bytes);
      await fd.sync();
    } finally {
      await fd.close();
    }
    if (immutable) {
      try {
        await link(tmp, file);
      } catch (e) {
        if ((e as NodeJS.ErrnoException).code === 'EEXIST') {
          if ((await readFile(file, 'utf8')) === bytes) {
            return;
          }
          fail('immutable_conflict', 'Immutable object already exists', 5);
        }
        throw e;
      }
    } else {
      await rename(tmp, file);
    }
  } finally {
    await unlink(tmp).catch(() => {});
  }
}

export class SpaceStore {
  readonly root: string;
  private lockToken?: string;
  constructor(readonly workspace: ResolvedWorkspace) {
    this.root = path.join(workspace.dataDir, 'spaces', workspace.definition.id);
  }
  assertSpace(workspace: ResolvedWorkspace) {
    if (
      workspace.definition.id !== this.workspace.definition.id ||
      workspace.workspaceDir !== this.workspace.workspaceDir
    ) {
      fail('workspace_mismatch', 'Store belongs to another space', 5);
    }
  }
  async withLock<T>(fn: () => Promise<T>): Promise<T> {
    await mkdir(this.root, { recursive: true });
    const lock = path.join(this.root, 'space.lock');
    let fd;
    try {
      fd = await open(lock, 'wx', 0o600);
    } catch (e) {
      if ((e as NodeJS.ErrnoException).code === 'EEXIST') {
        fail(
          'lock_conflict',
          'Space is locked; explicit recovery required for a dead owner',
          5,
        );
      }
      throw e;
    }
    try {
      await fd.writeFile(
        canonical({
          pid: process.pid,
          host: hostname(),
          token: (this.lockToken = randomUUID()),
          ownedGroups: [],
        }),
      );
      await fd.sync();
      return await fn();
    } finally {
      await fd.close();
      this.lockToken = undefined;
      await unlink(lock);
    }
  }
  async recoverLock(): Promise<void> {
    const file = path.join(this.root, 'space.lock');
    const lock = await readJson<{
      pid: number;
      host: string;
      ownedGroups?: { pgid: number }[];
    }>(file);
    if (lock.host !== hostname()) {
      fail('lock_owner_unknown', 'Cannot prove remote lock owner exited', 5);
    }
    try {
      process.kill(lock.pid, 0);
      fail('lock_owner_alive', 'Lock owner is still alive', 5);
    } catch (e) {
      if ((e as NodeJS.ErrnoException).code !== 'ESRCH') {
        throw e;
      }
    }
    for (const group of lock.ownedGroups ?? []) {
      try {
        process.kill(-group.pgid, 0);
        fail(
          'owned_process_alive',
          'Owned check process group is still alive; stop and confirm its exit before recovery',
          5,
        );
      } catch (e) {
        if ((e as NodeJS.ErrnoException).code !== 'ESRCH') {
          throw e;
        }
      }
    }
    await unlink(file);
  }
  async registerOwnedGroup(pgid: number, nonce: string): Promise<void> {
    if (!this.lockToken) {
      fail(
        'ownership_unavailable',
        'Process execution requires the active space lock',
        5,
      );
    }
    const file = path.join(this.root, 'space.lock');
    const lock = await readJson<{
      pid: number;
      host: string;
      token: string;
      ownedGroups: { pgid: number; nonce: string; recordedAt: string }[];
    }>(file);
    if (lock.token !== this.lockToken || lock.pid !== process.pid) {
      fail('lock_conflict', 'Process owner lock changed', 5);
    }
    lock.ownedGroups.push({
      pgid,
      nonce,
      recordedAt: new Date().toISOString(),
    });
    await atomicJson(file, lock);
  }
  async readCatalog(): Promise<Catalog> {
    return (
      (await maybeJson<Catalog>(
        path.join(this.workspace.workspaceDir, 'catalog.json'),
      )) ?? {
        schemaVersion: 1,
        workspaceId: this.workspace.definition.id,
        revision: 0,
        documents: [],
        annotations: [],
      }
    );
  }
  async writeCatalog(catalog: Catalog) {
    if (catalog.workspaceId !== this.workspace.definition.id) {
      fail('workspace_mismatch', 'Catalog space mismatch', 5);
    }
    await atomicJson(
      path.join(this.workspace.workspaceDir, 'catalog.json'),
      catalog,
    );
  }
  async readTask(id: string): Promise<TaskState> {
    const state = await readJson<TaskState>(
      path.join(this.root, 'tasks', safeId(id), 'state.json'),
    );
    if (state.workspaceId !== this.workspace.definition.id) {
      fail('workspace_mismatch', 'Task belongs to another space', 5);
    }
    return state;
  }
  async writeTask(state: TaskState, expectedRevision: number) {
    const old = await maybeJson<TaskState>(
      path.join(this.root, 'tasks', safeId(state.id), 'state.json'),
    );
    if ((old?.revision ?? 0) !== expectedRevision) {
      fail('revision_conflict', 'Task revision changed', 5);
    }
    await atomicJson(
      path.join(this.root, 'tasks', state.id, 'state.json'),
      state,
    );
  }
  async readContract(id: string, revision: number): Promise<TaskContract> {
    return readJson(
      path.join(
        this.workspace.workspaceDir,
        'tasks',
        safeId(id),
        'contracts',
        `${revision}.json`,
      ),
    );
  }
  async readRun(id: string): Promise<RunRecord> {
    if (!/^run_[a-f0-9-]+$/.test(id)) {
      fail('invalid_id', 'Invalid run ID');
    }
    return readJson(path.join(this.root, 'runs', id, 'record.json'));
  }
  async readIndex(revision?: string): Promise<ScanResult | undefined> {
    const pointer =
      revision ??
      (
        await maybeJson<{ revision: string }>(
          path.join(this.root, 'indexes', 'current.json'),
        )
      )?.revision;
    if (!pointer) {
      return;
    }
    if (!/^[a-f0-9]{64}$/.test(pointer)) {
      fail('invalid_revision', 'Invalid scan revision');
    }
    return maybeJson(path.join(this.root, 'indexes', pointer + '.json'));
  }
  async readTaskStates(): Promise<TaskState[]> {
    let dirs;
    try {
      dirs = await readdir(path.join(this.root, 'tasks'));
    } catch (e) {
      if ((e as NodeJS.ErrnoException).code === 'ENOENT') {
        return [];
      }
      throw e;
    }
    const states = [];
    for (const id of dirs) {
      states.push(await this.readTask(id));
    }
    return states;
  }
  async readCheckpoint(id: string): Promise<Checkpoint> {
    if (!/^cp_[a-f0-9-]+$/.test(id)) {
      fail('invalid_id', 'Invalid checkpoint ID');
    }
    return readJson(path.join(this.root, 'checkpoints', id, 'checkpoint.json'));
  }
}
export async function openSpaceStore(
  workspace: ResolvedWorkspace,
): Promise<SpaceStore> {
  const store = new SpaceStore(workspace);
  await mkdir(store.root, { recursive: true, mode: 0o700 });
  await atomicJson(
    path.join(store.root, 'owner.json'),
    {
      schemaVersion: 1,
      workspaceId: workspace.definition.id,
      workspaceDir: workspace.workspaceDir,
    },
    true,
  );
  return store;
}

export async function putObject(
  store: { readonly root: string },
  bytes: Buffer | string,
): Promise<string> {
  const id = hash(bytes);
  const target = path.join(store.root, 'objects', id);
  await mkdir(path.dirname(target), { recursive: true });
  const temporary = target + '.tmp-' + randomUUID();
  try {
    const fd = await open(temporary, 'wx', 0o600);
    try {
      await fd.writeFile(bytes);
      await fd.sync();
    } finally {
      await fd.close();
    }
    try {
      await link(temporary, target);
    } catch (e) {
      if ((e as NodeJS.ErrnoException).code !== 'EEXIST') {
        throw e;
      }
      if (hash(await readFile(target)) !== id) {
        fail('corrupt_object', 'Stored content hash mismatch', 3);
      }
    }
  } finally {
    await unlink(temporary).catch(() => {});
  }
  return 'objects/' + id;
}
export async function readObject(
  store: { readonly root: string },
  objectPath: string,
  expectedHash: string,
): Promise<Buffer> {
  if (!/^objects\/[a-f0-9]{64}$/.test(objectPath)) {
    fail('invalid_object', 'Invalid content object');
  }
  const bytes = await readFile(path.join(store.root, objectPath));
  if (hash(bytes) !== expectedHash) {
    fail('corrupt_object', 'Content object hash mismatch', 3);
  }
  return bytes;
}
