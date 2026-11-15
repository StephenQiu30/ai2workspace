import { openSpaceStore } from '@ai2workspace/core/workspace';
import Link from 'next/link';
import { notFound } from 'next/navigation';
import { readWorkspace } from '@/lib/workspaces';
import { SpaceShell } from '@/components/space-shell';
import { Badge } from '@/components/ui/badge';
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '@/components/ui/card';
import { Alert, AlertTitle, AlertDescription } from '@/components/ui/alert';
import { buttonVariants } from '@/components/ui/button';
export const dynamic = 'force-dynamic';
export default async function Page({ params }: PageProps<'/spaces/[space]'>) {
  const { space } = await params;
  const view = await readWorkspace(space).catch(() => notFound());
  const store = await openSpaceStore(view.workspace);
  const states = await store.readTaskStates();
  const tasks = await Promise.all(
    states.map(async (state) => ({
      state,
      checkpoint: state.checkpointId
        ? await store.readCheckpoint(state.checkpointId)
        : undefined,
      runs: await Promise.all(state.runIds.map((id) => store.readRun(id))),
    })),
  );
  return (
    <SpaceShell
      title={view.workspace.definition.name}
      description="治理问题、任务和交接来自本机 Core；网页只展示记录。"
    >
      <div className="flex flex-wrap gap-2">
        <Link
          className={buttonVariants({ variant: 'outline' })}
          href={`/spaces/${space}/read`}
        >
          阅读 {view.scan.documents.length} 份知识
        </Link>
        <Link
          className={buttonVariants({ variant: 'ghost' })}
          href={`/spaces/${space}/settings`}
        >
          编辑空间声明
        </Link>
        <Badge variant="secondary">{view.workspace.definition.profile}</Badge>
      </div>
      <section className="flex flex-col gap-3" aria-labelledby="issues">
        <h2 id="issues" className="text-xl font-semibold">
          治理问题
        </h2>
        <p className="break-all text-sm text-muted-foreground">
          知识修订：{view.scan.sourceRevision}
        </p>
        {view.scan.issues.length ? (
          view.scan.issues.map((issue, index) => (
            <Alert key={index}>
              <AlertTitle>
                {issue.code} · {issue.severity}
              </AlertTitle>
              <AlertDescription>
                {issue.message}
                {issue.path ? <span>{issue.path}</span> : null}
              </AlertDescription>
            </Alert>
          ))
        ) : (
          <p>当前扫描未报告治理问题。</p>
        )}
      </section>
      <section className="flex flex-col gap-4" aria-labelledby="tasks">
        <h2 id="tasks" className="text-xl font-semibold">
          任务与交接
        </h2>
        <p className="text-sm text-muted-foreground">
          记录结果不代表当前候选仍通过；接续前由 Codex 调用 CLI 核对。
        </p>
        {!tasks.length ? (
          <p>尚无任务记录。</p>
        ) : (
          tasks.map(({ state, checkpoint, runs }) => (
            <Card key={state.id}>
              <CardHeader>
                <CardTitle>{state.id}</CardTitle>
                <CardDescription>
                  合同 r{state.contractRevision} · 状态 r{state.revision}
                </CardDescription>
              </CardHeader>
              <CardContent className="flex flex-col gap-4">
                <div className="flex flex-wrap gap-2">
                  <Badge>{state.phase}</Badge>
                  <Badge variant="outline">{state.condition}</Badge>
                </div>
                {state.issues.map((issue, index) => (
                  <p key={index}>
                    {issue.code}：{issue.message}
                  </p>
                ))}
                <ul className="flex flex-col gap-2">
                  {runs.map((run) => (
                    <li key={run.runId} className="break-all text-sm">
                      {run.checkId} · 记录结果 {run.result} ·{' '}
                      {run.evidenceSource} · {run.runId}
                    </li>
                  ))}
                </ul>
                {checkpoint ? (
                  <div className="flex flex-col gap-3">
                    <h3 className="font-semibold">
                      交接 · {checkpoint.createdAt}
                    </h3>
                    <p>{checkpoint.goal}</p>
                    <p className="text-sm text-muted-foreground">
                      叙述主张按原样展示，未自动转为通过证据。
                    </p>
                    <ul>
                      {checkpoint.notes.completedClaims.map((claim, index) => (
                        <li key={index}>
                          {claim.text}
                          {claim.evidenceRunIds?.length
                            ? `（引用 ${claim.evidenceRunIds.join('、')}）`
                            : '（无证据引用）'}
                        </li>
                      ))}
                    </ul>
                    <h4 className="font-medium">未决事项</h4>
                    <ul>
                      {checkpoint.notes.openIssues.map((issue, index) => (
                        <li key={index}>{issue}</li>
                      ))}
                    </ul>
                    <h4 className="font-medium">下一步</h4>
                    <ol>
                      {checkpoint.notes.nextSteps.map((step, index) => (
                        <li key={index}>{step}</li>
                      ))}
                    </ol>
                    <h4 className="font-medium">外部动作</h4>
                    <ul>
                      {checkpoint.notes.externalActions.map((action, index) => (
                        <li key={index}>
                          {action.status}：{action.description}
                        </li>
                      ))}
                    </ul>
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    尚无完整交接。
                  </p>
                )}
              </CardContent>
            </Card>
          ))
        )}
      </section>
    </SpaceShell>
  );
}
