#!/usr/bin/env node
import { readFile } from 'node:fs/promises';
import { parseArgs } from 'node:util';
import {
  resolveWorkspace,
  openSpaceStore,
  CoreError,
  type Issue,
} from '@ai2workspace/core/workspace';
import {
  scanSources,
  annotateCatalog,
  type CatalogAnnotationInput,
} from '@ai2workspace/core/knowledge';
import {
  createTask,
  reviseTask,
  prepareTask,
  exportContext,
  runCheck,
  verifyTask,
  createHandoff,
  resumeTask,
  completeTask,
  type TaskInput,
  type TaskRevisionInput,
  type HandoffNotes,
} from '@ai2workspace/core/harness';

const help = `ai2workspace — 本机知识治理与 Codex 交接

ai2workspace --workspace <目录> <命令> [参数] [--json]

  space validate
  space recover-lock
  source scan
  catalog annotate --input <JSON> --reason <说明>
  task create --contract <JSON>
  task revise --task <ID> --contract <JSON> --reason <说明>
  task prepare --task <ID> [--rebaseline --reason <说明>]
  context export --task <ID>
  check run --task <ID> --check <ID>
  task verify --task <ID>
  task handoff --task <ID> --notes <JSON>
  task resume --task <ID>
  task complete --task <ID>
  task status --task <ID>
  task list

退出码：0 操作成功；1 检查失败；2 输入错误；3 前置阻塞；4 证据过期；5 冲突。
查询/交接命令成功不代表任务通过；以上命令不运行编码 Agent，也不执行 Git 发布。`;

const routes: Record<string, string[]> = {
  'space validate': [],
  'space recover-lock': [],
  'source scan': [],
  'catalog annotate': ['input', 'reason'],
  'task create': ['contract'],
  'task revise': ['task', 'contract', 'reason'],
  'task prepare': ['task', 'rebaseline', 'reason', 'late-prepare'],
  'context export': ['task'],
  'check run': ['task', 'check'],
  'task verify': ['task'],
  'task handoff': ['task', 'notes'],
  'task resume': ['task'],
  'task complete': ['task'],
  'task status': ['task'],
  'task list': [],
};

function invalid(message: string): never {
  throw new CoreError(2, [
    { code: 'invalid_arguments', severity: 'error', message },
  ]);
}

async function jsonInput<T>(file: string): Promise<T> {
  let contents: string;
  try {
    contents = await readFile(file, 'utf8');
  } catch {
    throw new CoreError(3, [
      {
        code: 'input_unavailable',
        severity: 'error',
        message: '无法读取指定输入文件',
      },
    ]);
  }
  try {
    return JSON.parse(contents) as T;
  } catch {
    return invalid('输入文件必须是有效 JSON');
  }
}

async function main(): Promise<void> {
  let operation = '';
  let workspaceId: string | null = null;
  let taskId: string | undefined;
  let useJson = process.argv.includes('--json');
  function emit(data: unknown, code = 0, issues: Issue[] = []) {
    const envelope = {
      schemaVersion: 1,
      ok: code === 0,
      operation,
      workspaceId,
      taskId,
      data,
      issues,
    };
    process.stdout.write(
      useJson
        ? JSON.stringify(envelope) + '\n'
        : JSON.stringify(envelope, null, 2) + '\n',
    );
    process.exitCode = code;
  }
  try {
    const parsed = parseArgs({
      allowPositionals: true,
      strict: true,
      tokens: true,
      options: {
        workspace: { type: 'string' },
        json: { type: 'boolean' },
        help: { type: 'boolean', short: 'h' },
        task: { type: 'string' },
        check: { type: 'string' },
        contract: { type: 'string' },
        input: { type: 'string' },
        reason: { type: 'string' },
        notes: { type: 'string' },
        rebaseline: { type: 'boolean' },
        'late-prepare': { type: 'boolean' },
      },
    });
    useJson = parsed.values.json === true;
    const seen = new Set<string>();
    for (const token of parsed.tokens) {
      if (token.kind === 'option') {
        if (seen.has(token.name)) {
          invalid(`参数 --${token.name} 不可重复`);
        }
        seen.add(token.name);
      }
    }
    if (parsed.values.help) {
      if (useJson) {
        emit({ help });
      } else {
        process.stdout.write(help + '\n');
      }
      return;
    }
    operation = parsed.positionals.join(' ');
    if (!(operation in routes)) {
      invalid('未知命令，请使用 --help 查看支持的操作');
    }
    for (const key of Object.keys(parsed.values)) {
      if (!['workspace', 'json', 'help', ...routes[operation]].includes(key)) {
        invalid(`当前命令不支持 --${key}`);
      }
    }
    const required = (key: keyof typeof parsed.values): string => {
      const value = parsed.values[key];
      if (typeof value !== 'string' || value.length === 0) {
        return invalid(`缺少 --${key}`);
      }
      return value;
    };
    const workspace = await resolveWorkspace(required('workspace'));
    workspaceId = workspace.definition.id;
    taskId =
      typeof parsed.values.task === 'string' ? parsed.values.task : undefined;
    const store = await openSpaceStore(workspace);
    switch (operation) {
      case 'space validate':
        emit(
          {
            workspaceId,
            profile: workspace.definition.profile,
            sources: workspace.definition.sources.map((source) => ({
              id: source.id,
              kind: source.kind,
            })),
            projects: workspace.definition.projects.map(
              (project) => project.id,
            ),
          },
          workspace.issues.some((issue) => issue.severity === 'error') ? 3 : 0,
          workspace.issues,
        );
        break;
      case 'space recover-lock':
        await store.recoverLock();
        emit({ recovered: true });
        break;
      case 'source scan':
        emit(await scanSources(workspace, store));
        break;
      case 'catalog annotate': {
        const annotation = await jsonInput<CatalogAnnotationInput>(
          required('input'),
        );
        emit(
          await annotateCatalog(workspace, store, {
            ...annotation,
            reason: required('reason'),
          }),
        );
        break;
      }
      case 'task create':
        emit(
          await createTask(
            workspace,
            store,
            await jsonInput<TaskInput>(required('contract')),
          ),
        );
        break;
      case 'task revise': {
        const id = required('task');
        const input = await jsonInput<TaskInput | TaskRevisionInput>(
          required('contract'),
        );
        const revision =
          'contract' in input
            ? { ...input, reason: required('reason') }
            : {
                contract: input,
                expectedRevision: (await store.readTask(id)).contractRevision,
                reason: required('reason'),
              };
        emit(await reviseTask(workspace, store, id, revision));
        break;
      }
      case 'task prepare': {
        const result = await prepareTask(workspace, store, required('task'), {
          rebaseline: parsed.values.rebaseline === true,
          reason: parsed.values.reason,
          latePrepare: parsed.values['late-prepare'] === true,
        });
        const state = await store.readTask(required('task'));
        emit(
          result,
          result.ready ? 0 : state.condition === 'needs_rebaseline' ? 4 : 3,
          result.issues,
        );
        break;
      }
      case 'context export':
        emit(await exportContext(workspace, store, required('task')));
        break;
      case 'check run': {
        const result = await runCheck(
          workspace,
          store,
          required('task'),
          required('check'),
        );
        emit(
          result,
          result.result === 'passed' ? 0 : result.result === 'blocked' ? 3 : 1,
          result.issues,
        );
        break;
      }
      case 'task verify': {
        const result = await verifyTask(workspace, store, required('task'));
        const satisfied =
          result.condition === 'clear' &&
          ['verified', 'complete'].includes(result.phase);
        const code = satisfied
          ? 0
          : result.condition === 'needs_rebaseline' ||
              result.staleRunIds.length > 0
            ? 4
            : result.condition === 'blocked'
              ? 3
              : 1;
        emit(result, code, result.issues);
        break;
      }
      case 'task handoff':
        emit(
          await createHandoff(
            workspace,
            store,
            required('task'),
            await jsonInput<HandoffNotes>(required('notes')),
          ),
        );
        break;
      case 'task resume':
        emit(await resumeTask(workspace, store, required('task')));
        break;
      case 'task complete':
        emit(await completeTask(workspace, store, required('task')));
        break;
      case 'task status':
        emit(await store.readTask(required('task')));
        break;
      case 'task list':
        emit(await store.readTaskStates());
        break;
    }
  } catch (error) {
    if (error instanceof CoreError) {
      emit(null, error.exitCode, error.issues);
    } else if (
      error instanceof TypeError &&
      'code' in error &&
      String(error.code).startsWith('ERR_PARSE_ARGS')
    ) {
      emit(null, 2, [
        {
          code: 'invalid_arguments',
          severity: 'error',
          message: '参数格式无效，请查看 --help',
        },
      ]);
    } else {
      const missing =
        error instanceof Error && 'code' in error && error.code === 'ENOENT';
      emit(null, missing ? 3 : 1, [
        {
          code: missing ? 'resource_unavailable' : 'operation_failed',
          severity: 'error',
          message: error instanceof Error ? error.message : '操作失败',
        },
      ]);
    }
  }
}

await main();
