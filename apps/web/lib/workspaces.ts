import 'server-only';
import { readdir, realpath, lstat, readFile } from 'node:fs/promises';
import path from 'node:path';
import { cache } from 'react';
import { parseDocument } from 'yaml';
import {
  resolveWorkspace,
  openSpaceStore,
  type ResolvedWorkspace,
} from '@ai2workspace/core/workspace';
import { scanSources, type ScanResult } from '@ai2workspace/core/knowledge';

export const spacesRoot = path.resolve(
  /* turbopackIgnore: true */
  process.env.AI2WORKSPACE_SPACES_DIR ??
    path.join(process.cwd(), '../../spaces'),
);
export const spaceKeyPattern = /^[a-z0-9][a-z0-9-]{0,63}$/;

export async function registeredSpacePath(key: string): Promise<string> {
  if (!spaceKeyPattern.test(key)) {
    throw new Error('无效空间标识');
  }
  const root = await realpath(/* turbopackIgnore: true */ spacesRoot);
  const candidate = path.join(root, key);
  const stat = await lstat(candidate);
  if (!stat.isDirectory() || stat.isSymbolicLink()) {
    throw new Error('空间目录必须是普通目录');
  }
  const directory = await realpath(candidate);
  const definition = await lstat(path.join(directory, 'workspace.yaml'));
  if (!definition.isFile() || definition.isSymbolicLink()) {
    throw new Error('空间声明必须是普通文件');
  }
  if (path.dirname(directory) !== root) {
    throw new Error('空间配置目录越界');
  }
  return directory;
}

export const listWorkspaces = cache(async () => {
  const entries = await readdir(/* turbopackIgnore: true */ spacesRoot, {
    withFileTypes: true,
  }).catch(() => []);
  const registered = await Promise.all(
    entries.map(async (entry) =>
      entry.isDirectory() &&
      spaceKeyPattern.test(entry.name) &&
      (await lstat(path.join(spacesRoot, entry.name, 'workspace.yaml'))
        .then((stat) => stat.isFile() && !stat.isSymbolicLink())
        .catch(() => false))
        ? entry
        : undefined,
    ),
  );
  const spaces = await Promise.all(
    registered
      .filter((entry) => entry !== undefined)

      .map(async (entry) => {
        let declaredId: string | undefined;
        try {
          const yaml = parseDocument(
            await readFile(
              path.join(spacesRoot, entry.name, 'workspace.yaml'),
              'utf8',
            ),
          );
          const id: unknown = yaml.get('id');
          if (typeof id === 'string' && spaceKeyPattern.test(id)) {
            declaredId = id;
          }
          const workspace = await resolveWorkspace(
            await registeredSpacePath(entry.name),
          );
          return {
            key: entry.name,
            name: workspace.definition.name,
            id: workspace.definition.id,
            issue: undefined,
          };
        } catch {
          return {
            key: entry.name,
            name: entry.name,
            id: declaredId,
            issue: '配置或本机绑定不可用',
          };
        }
      }),
  );
  return spaces.map((space) => ({
    ...space,
    issue:
      space.id &&
      spaces.some((other) => other.key !== space.key && other.id === space.id)
        ? '空间 ID 重复，读取已阻止，请在本机修复登记'
        : space.issue,
  }));
});

export const resolveRegisteredWorkspace = cache(async (key: string) => {
  const workspace = await resolveWorkspace(await registeredSpacePath(key));
  const registered = await listWorkspaces();
  if (
    registered.some(
      (other) => other.key !== key && other.id === workspace.definition.id,
    )
  ) {
    throw new Error('空间 ID 重复，读取已阻止');
  }
  return workspace;
});

export interface WorkspaceView {
  key: string;
  workspace: ResolvedWorkspace;
  scan: ScanResult;
  stale: boolean;
}

// Only de-duplicate concurrent scans. The immutable revision store is owned by Core.
const pendingScans = new Map<string, Promise<ScanResult>>();
export const readWorkspace = cache(
  async (key: string, revision?: string): Promise<WorkspaceView> => {
    const workspace = await resolveRegisteredWorkspace(key);
    const store = await openSpaceStore(workspace);
    let pending = pendingScans.get(workspace.workspaceDir);
    if (!pending) {
      pending = scanSources(workspace, store);
      pendingScans.set(workspace.workspaceDir, pending);
      void pending
        .finally(() => pendingScans.delete(workspace.workspaceDir))
        .catch(() => {});
    }
    const scan = await pending.catch(async (error: unknown) => {
      const previous = await store.readIndex();
      if (!previous) {
        throw error;
      }
      return {
        ...previous,
        issues: [
          ...previous.issues,
          {
            code: 'stale-index',
            severity: 'warning' as const,
            message: '当前扫描未完成，展示上次完整知识修订。',
          },
        ],
      };
    });
    if (revision && scan.sourceRevision !== revision) {
      // Historical lookup is supplied by the shared Core store adapter.
      if (!/^[a-f0-9]{64}$/.test(revision)) {
        throw new Error('无效知识修订');
      }
      const historical = await store.readIndex(revision);
      if (!historical || historical.workspaceId !== workspace.definition.id) {
        throw new Error('知识修订不存在');
      }
      return { key, workspace, scan: historical, stale: true };
    }
    return {
      key,
      workspace,
      scan,
      stale: scan.issues.some((issue) =>
        /unavailable|missing|offline|stale/i.test(issue.code),
      ),
    };
  },
);
