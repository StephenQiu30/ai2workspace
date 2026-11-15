import { spawn } from 'node:child_process';
import { mkdir, writeFile, readFile, lstat } from 'node:fs/promises';
import path from 'node:path';
import { randomUUID } from 'node:crypto';
import type { ResolvedWorkspace, CheckDefinition } from '../workspace/types.js';
import type { ToolProof, RunRecord } from '../harness/types.js';
import type { Issue } from '../shared/types.js';
import {
  SpaceStore,
  digest,
  hash,
  contained,
  atomicJson,
  fail,
} from '../storage/file-store.js';
import { fingerprint, contextDrift, readManifest } from './preparation.js';
interface ProcessResult {
  stdout: string;
  stderr: string;
  exitCode: number | null;
  signal: string | null;
  timedOut: boolean;
  error?: string;
}
function redact(text: string): string {
  return text
    .replace(
      /((?:api[_-]?key|token|secret|password)\s*[:=]\s*)[^\s"']+/gi,
      '$1[REDACTED]',
    )
    .replace(
      /-----BEGIN [\s\S]*?PRIVATE KEY-----[\s\S]*?-----END [\s\S]*?PRIVATE KEY-----/g,
      '[REDACTED PRIVATE KEY]',
    );
}
// The bootstrap waits for a persisted ownership receipt before launching the check.
// A disconnected pre-launch bootstrap exits without executing the supplied argv.
const bootstrap = `
 const {spawn}=await import('node:child_process');
 let started=false;
 process.on('disconnect',()=>{if(!started)process.exit(125)});
 process.once('message',message=>{
  if(message.nonce!==process.env.AI2WORKSPACE_PROCESS_NONCE)process.exit(126);
  started=true;
  const env={...process.env};delete env.AI2WORKSPACE_PROCESS_NONCE;
  const target=spawn(message.argv[0],message.argv.slice(1),{shell:false,cwd:process.cwd(),env,stdio:['ignore','inherit','inherit']});
  target.on('error',error=>{process.send?.({error:error.code??error.message});process.exitCode=127});
  target.on('close',(code,signal)=>{if(signal)process.kill(process.pid,signal);else process.exit(code??127)});
 });
`;
export async function execute(
  argv: string[],
  cwd: string,
  timeoutMs: number,
  env: NodeJS.ProcessEnv,
  owner?: SpaceStore,
  runDir?: string,
): Promise<ProcessResult> {
  return new Promise((resolve) => {
    let stdout = '';
    let stderr = '';
    let timedOut = false;
    let error: string | undefined;
    let signal: string | null = null;
    const nonce = randomUUID();
    const child = spawn(
      process.execPath,
      ['--input-type=module', '-e', bootstrap],
      {
        cwd,
        env: { ...env, AI2WORKSPACE_PROCESS_NONCE: nonce },
        shell: false,
        detached: process.platform !== 'win32',
        stdio: ['ignore', 'pipe', 'pipe', 'ipc'],
      },
    );
    let settled = false;
    const kill = () => {
      try {
        if (process.platform !== 'win32' && child.pid) {
          process.kill(-child.pid, 'SIGKILL');
        } else {
          child.kill('SIGKILL');
        }
      } catch {
        /* Group already exited. */
      }
    };
    const cancel = () => {
      error = 'cancelled';
      kill();
    };
    process.once('SIGINT', cancel);
    process.once('SIGTERM', cancel);
    const timer = setTimeout(() => {
      timedOut = true;
      kill();
    }, timeoutMs);
    const collect = (current: string, chunk: Buffer) => {
      if (Buffer.byteLength(current) + chunk.length > 8 * 1024 * 1024) {
        error = 'output_limit';
        kill();
        return current;
      }
      return current + chunk.toString('utf8');
    };
    child.stdout!.on('data', (chunk: Buffer) => {
      stdout = collect(stdout, chunk);
    });
    child.stderr!.on('data', (chunk: Buffer) => {
      stderr = collect(stderr, chunk);
    });
    child.on('message', (message) => {
      if (message && typeof message === 'object' && 'error' in message) {
        error = String(message.error);
      }
    });
    child.once('spawn', () => {
      void (async () => {
        try {
          if (!child.pid) {
            throw Error('no pid');
          }
          if (owner) {
            await owner.registerOwnedGroup(child.pid, nonce);
          }
          if (runDir) {
            await atomicJson(
              path.join(runDir, 'process.json'),
              { pgid: child.pid, nonce, ownerPid: process.pid },
              true,
            );
          }
          if (!settled && !timedOut && child.connected) {
            child.send({ argv, nonce }, (e) => {
              if (e) {
                error = 'ownership_channel_closed';
                kill();
              }
            });
          }
        } catch {
          error = 'ownership_unavailable';
          kill();
        }
      })();
    });
    child.on('error', (e) => {
      error = (e as NodeJS.ErrnoException).code ?? e.message;
    });
    child.on('close', (exitCode, sig) => {
      if (settled) {
        return;
      }
      settled = true;
      signal = sig;
      clearTimeout(timer);
      kill();
      process.off('SIGINT', cancel);
      process.off('SIGTERM', cancel);
      resolve({
        stdout: redact(stdout),
        stderr: redact(stderr),
        exitCode,
        signal,
        timedOut,
        error,
      });
    });
  });
}
export async function probe(
  workspace: ResolvedWorkspace,
  check: CheckDefinition,
  store?: SpaceStore,
): Promise<{
  tools: ToolProof[];
  environment: Record<string, string>;
  issues: Issue[];
}> {
  const issues: Issue[] = [];
  const tools: ToolProof[] = [];
  const environment: Record<string, string> = {};
  if (check.environment.privateRefs.length) {
    issues.push({
      code: 'unsupported_environment',
      severity: 'error',
      message:
        'Private environment references are unsupported in the first loop',
    });
    return { tools, environment, issues };
  }
  const project = workspace.definition.projects.find(
    (p) => p.id === check.projectId,
  )!;
  const root = workspace.bindings[project.binding];
  if (!root) {
    return {
      tools,
      environment,
      issues: [
        {
          code: 'missing_binding',
          severity: 'error',
          message: 'Check project binding unavailable',
        },
      ],
    };
  }
  let cwd: string;
  try {
    cwd = await contained(root, check.cwd);
  } catch {
    return {
      tools,
      environment,
      issues: [
        {
          code: 'invalid_cwd',
          severity: 'error',
          message: 'Check cwd is unavailable or unsafe',
        },
      ],
    };
  }
  for (const tool of check.tools) {
    const result = await execute(tool.argv, cwd, 5000, process.env, store);
    tools.push({
      id: tool.id,
      argv: tool.argv,
      outputHash: digest({ stdout: result.stdout, stderr: result.stderr }),
      exitCode: result.exitCode,
    });
    if (result.error || result.timedOut || result.exitCode !== 0) {
      issues.push({
        code: 'tool_unavailable',
        severity: 'error',
        message: `Tool probe failed: ${tool.id}`,
      });
    }
  }
  for (const name of check.environment.publicNames) {
    environment[name] =
      process.env[name] === undefined ? 'missing' : hash(process.env[name]!);
  }
  return { tools, environment, issues };
}
export async function runCheck(
  workspace: ResolvedWorkspace,
  store: SpaceStore,
  taskId: string,
  checkId: string,
): Promise<RunRecord> {
  store.assertSpace(workspace);
  return store.withLock(async () => {
    let state = await store.readTask(taskId);
    if (!state.snapshotId) {
      fail('missing_snapshot', 'Prepare task before checks', 3);
    }
    if (['complete', 'cancelled'].includes(state.phase)) {
      fail('terminal_task', 'Terminal task cannot run checks', 3);
    }
    const contract = await store.readContract(taskId, state.contractRevision);
    if (!contract.checkIds.includes(checkId)) {
      fail('undeclared_check', 'Check is not declared by task');
    }
    const check = workspace.definition.checks.find((c) => c.id === checkId);
    if (!check) {
      fail('missing_check', 'Declared check unavailable', 3);
    }
    const basisIssues = await contextDrift(
      workspace,
      store,
      contract,
      await readManifest(store, state.snapshotId),
    );
    if (basisIssues.length) {
      await store.writeTask(
        {
          ...state,
          revision: state.revision + 1,
          condition: 'needs_rebaseline',
          issues: basisIssues,
        },
        state.revision,
      );
      fail('stale_context', 'Task context requires rebaseline', 4);
    }
    const before = await fingerprint(workspace, check.projectId);
    const proof = await probe(workspace, check, store);
    const runId = 'run_' + randomUUID();
    const runDir = path.join(store.root, 'runs', runId);
    await mkdir(path.join(runDir, 'artifacts'), {
      recursive: true,
      mode: 0o700,
    });
    const startedAt = new Date().toISOString();
    await atomicJson(
      path.join(runDir, 'started.json'),
      {
        runId,
        taskId,
        workspaceId: workspace.definition.id,
        contractRevision: contract.revision,
        snapshotId: state.snapshotId,
        startedAt,
      },
      true,
    );
    await store.writeTask(
      {
        ...state,
        revision: state.revision + 1,
        phase: 'verifying',
        condition: 'clear',
      },
      state.revision,
    );
    state = await store.readTask(taskId);
    const issues: Issue[] = [...proof.issues];
    const project = workspace.definition.projects.find(
      (p) => p.id === check.projectId,
    )!;
    let cwd = check.cwd;
    let processResult: ProcessResult = {
      stdout: '',
      stderr: '',
      exitCode: null,
      signal: null,
      timedOut: false,
    };
    if (!issues.length) {
      try {
        cwd = await contained(workspace.bindings[project.binding]!, check.cwd);
        processResult = await execute(
          check.argv,
          cwd,
          check.timeoutMs,
          {
            ...process.env,
            AI2WORKSPACE_RUN_DIR: runDir,
          },
          store,
          runDir,
        );
      } catch {
        issues.push({
          code: 'invalid_cwd',
          severity: 'error',
          message: 'Check cwd is unavailable',
        });
      }
    }
    if (processResult.error) {
      issues.push({
        code:
          processResult.error === 'ENOENT'
            ? 'missing_executable'
            : 'process_error',
        severity: 'error',
        message: `Check process failed: ${processResult.error}`,
      });
    }
    if (processResult.timedOut) {
      issues.push({
        code: 'timeout',
        severity: 'error',
        message: 'Check exceeded timeout; owned process group terminated',
      });
    }
    const after = await fingerprint(workspace, check.projectId);
    if (before.hash !== after.hash) {
      issues.push({
        code: 'input_changed',
        severity: 'error',
        message: 'Check changed candidate inputs',
      });
    }
    const artifacts: RunRecord['artifacts'] = [];
    if (!proof.issues.length) {
      for (const rel of check.artifacts) {
        try {
          const target = await contained(path.join(runDir, 'artifacts'), rel);
          if (!(await lstat(target)).isFile()) {
            throw Error('not a file');
          }
          artifacts.push({
            path: 'artifacts/' + rel,
            hash: hash(await readFile(target)),
          });
        } catch {
          issues.push({
            code: 'missing_artifact',
            severity: 'error',
            message: 'Required current-run artifact missing or unsafe',
            path: rel,
          });
        }
      }
    }
    await writeFile(path.join(runDir, 'stdout.log'), processResult.stdout, {
      mode: 0o600,
    });
    await writeFile(path.join(runDir, 'stderr.log'), processResult.stderr, {
      mode: 0o600,
    });
    const blocked =
      proof.issues.length > 0 ||
      issues.some((i) =>
        ['missing_executable', 'invalid_cwd'].includes(i.code),
      );
    const result: RunRecord = {
      schemaVersion: 1,
      runId,
      workspaceId: workspace.definition.id,
      taskId,
      contractRevision: contract.revision,
      snapshotId: state.snapshotId!,
      checkId,
      checkHash: digest(check),
      projectId: check.projectId,
      bindingRevision: before.bindingRevision,
      candidateBefore: before,
      candidateAfter: after,
      argv: check.argv,
      cwd: check.cwd,
      tools: proof.tools,
      environment: proof.environment,
      environmentScope:
        'Declared public variables and tools only; undeclared inherited environment is outside proof',
      startedAt,
      endedAt: new Date().toISOString(),
      exitCode: processResult.exitCode,
      signal: processResult.signal,
      timedOut: processResult.timedOut,
      result: blocked
        ? 'blocked'
        : processResult.exitCode === 0 && issues.length === 0
          ? 'passed'
          : 'failed',
      issues,
      stdoutHash: hash(processResult.stdout),
      stderrHash: hash(processResult.stderr),
      artifacts,
      evidenceSource: 'cli-captured',
      sealed: true,
    };
    await atomicJson(path.join(runDir, 'record.json'), result, true);
    await store.writeTask(
      {
        ...state,
        revision: state.revision + 1,
        runIds: [...state.runIds, runId],
        condition:
          result.result === 'passed'
            ? 'clear'
            : result.result === 'blocked'
              ? 'blocked'
              : 'failed',
        issues,
      },
      state.revision,
    );
    return result;
  });
}
