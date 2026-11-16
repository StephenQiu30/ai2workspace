'use server';

import { parseDocument } from 'yaml';
import {
  CoreError,
  validateWorkspaceDefinition,
  updateWorkspaceDefinition,
} from '@ai2workspace/core/workspace';
import { revalidatePath } from 'next/cache';
import { registeredSpacePath, spaceKeyPattern } from './workspaces';

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
    validateWorkspaceDefinition(yaml.toJS({ maxAliasCount: 0 }));
    const creating = data.get('create') === 'true';
    if (creating) {
      return {
        success: false,
        message: '当前版本是项目内 Workspace 模块，请配置当前工作区。',
      };
    }
    await updateWorkspaceDefinition(
      await registeredSpacePath(key),
      text,
      String(data.get('revision') ?? ''),
    );
    revalidatePath('/spaces');
    revalidatePath(`/spaces/${key}/settings`);
    return {
      success: true,
      message: `Workspace ${key} 声明已保存。新声明在下一次读取时生效。`,
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
