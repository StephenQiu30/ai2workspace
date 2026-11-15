'use server';

import { writeFile, mkdir, rm, rmdir, realpath, open } from 'node:fs/promises';
import path from 'node:path';
import { hostname } from 'node:os';
import { parseDocument } from 'yaml';
import {
  CoreError,
  validateWorkspaceDefinition,
  updateWorkspaceDefinition,
} from '@ai2workspace/core/workspace';
import { revalidatePath } from 'next/cache';
import {
  registeredSpacePath,
  spacesRoot,
  spaceKeyPattern,
  listWorkspaces,
} from './workspaces';

export interface SaveState {
  message: string;
  success: boolean;
}

export async function saveDefinition(
  _state: SaveState,
  data: FormData,
): Promise<SaveState> {
  try {
    const key = String(data.get('space') ?? '');
    if (!spaceKeyPattern.test(key)) {
      return {
        success: false,
        message: '空间标识仅支持小写字母、数字和连字符',
      };
    }
    const text = String(data.get('definition') ?? '').replace(/\r\n/g, '\n');
    if (text.length > 100_000) {
      return { success: false, message: '配置超过长度限制' };
    }
    const yaml = parseDocument(text, { uniqueKeys: true });
    if (yaml.errors.length) {
      return { success: false, message: 'YAML 格式不正确' };
    }
    const definition = validateWorkspaceDefinition(
      yaml.toJS({ maxAliasCount: 0 }),
    );
    const creating = data.get('create') === 'true';
    if (creating) {
      await mkdir(/* turbopackIgnore: true */ spacesRoot, {
        recursive: true,
        mode: 0o700,
      });
      const root = await realpath(/* turbopackIgnore: true */ spacesRoot);
      const lockPath = path.join(root, '.registry-create.lock');
      const lock = await open(lockPath, 'wx', 0o600);
      try {
        await lock.writeFile(
          JSON.stringify({
            schemaVersion: 1,
            pid: process.pid,
            host: hostname(),
            purpose: 'registry-create',
          }),
        );
        await lock.sync();
        const spaces = await listWorkspaces();
        if (spaces.some((space) => space.id === definition.id)) {
          return {
            success: false,
            message: '空间 ID 已被登记，不能创建重复身份',
          };
        }
        const directory = path.join(root, key);
        await mkdir(directory, { recursive: false, mode: 0o700 });
        const ownedFiles: string[] = [];
        try {
          const target = path.join(directory, 'workspace.yaml');
          await writeFile(target, text, { mode: 0o600, flag: 'wx' });
          ownedFiles.push(target);
          const local = path.join(directory, 'workspace.local.yaml');
          await writeFile(
            local,
            'schemaVersion: 1\nbindings: {}\nautoScan: false\n',
            { mode: 0o600, flag: 'wx' },
          );
          ownedFiles.push(local);
        } catch (error) {
          await Promise.all(
            ownedFiles.map((file) => rm(file, { force: true })),
          );
          await rmdir(directory).catch(() => {});
          throw error;
        }
      } finally {
        await lock.close();
        await rm(lockPath, { force: true });
      }
    } else {
      await updateWorkspaceDefinition(
        await registeredSpacePath(key),
        text,
        String(data.get('revision') ?? ''),
      );
    }
    revalidatePath('/spaces');
    revalidatePath(`/spaces/${key}/settings`);
    return {
      success: true,
      message: `空间 ${key} 声明已保存。${creating ? '请在本机 workspace.local.yaml 中配置 bindings 后阅读。' : '新声明在下一次读取时生效。'}`,
    };
  } catch (error) {
    if (error instanceof CoreError) {
      const conflict = error.issues.some((issue) =>
        /revision_conflict|configuration_conflict/.test(issue.code),
      );
      return {
        success: false,
        message: conflict
          ? '配置已被其他操作更新，请刷新后重试'
          : '空间声明未通过 Core 校验或空间正被占用，请核对逻辑 ID、工程引用和当前锁状态',
      };
    }
    const code = (error as NodeJS.ErrnoException).code;
    return {
      success: false,
      message:
        code === 'EEXIST'
          ? '空间目录已存在或登记正在进行，请刷新后重试'
          : '声明保存失败，请检查 YAML 与本机空间配置',
    };
  }
}
