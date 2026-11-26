'use client';

import {Layout} from '@/components/layout/layout';
import {Heading, Text} from '@/components/layout/typography';
import {useState} from 'react';
import Link from 'next/link';
import {
  AlertCircleIcon,
  CheckCircle2Icon,
  FileTextIcon,
  PencilIcon,
  PlayIcon,
  PlusIcon,
  SendIcon,
  ShieldBanIcon,
} from 'lucide-react';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import {Badge} from '@/components/ui/badge';
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import {Input} from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {Textarea} from '@/components/ui/textarea';
import {
  extractBudgetPausedJob,
  formatCentsToYuan,
  getIndexJobStageLabel,
  getIndexJobStateLabel,
  getKnowledgeVisibilityLabel,
  getSourceKindLabel,
  getSourceStateLabel,
} from '@/features/assistant/assistant_helpers';
import {formatDateTime, getErrorMessage} from '@/lib/format';
import {
  assistantGetKnowledgeBase,
  assistantListKnowledgeSources,
  assistantPublishKnowledgeSource,
  assistantRegisterKnowledgeSource,
  assistantRevokeKnowledgeSource,
  assistantStartIndexJob,
  assistantUpdateKnowledgeSource,
} from '@/services/generated/api/knowledge';

export interface KnowledgeDetailViewProps {
  knowledgeBaseId: string;
  initialBase: Api.KnowledgeBase | null;
  initialSources: Api.KnowledgeSource[];
  initialError?: string | null;
}

export function KnowledgeDetailView({
  knowledgeBaseId,
  initialBase,
  initialSources,
  initialError,
}: KnowledgeDetailViewProps) {
  const [base, setBase] = useState<Api.KnowledgeBase | null>(initialBase);
  const [sources, setSources] = useState<Api.KnowledgeSource[]>(initialSources);
  const [errorMsg, setErrorMsg] = useState<string | null>(initialError ?? null);
  const [statusMsg, setStatusMsg] = useState<string | null>(null);
  const [jobsBySource, setJobsBySource] = useState<
    Record<string, Api.IndexJob>
  >({});
  const [busySourceId, setBusySourceId] = useState<string | null>(null);

  // Step 1: Register new source dialog
  const [registerOpen, setRegisterOpen] = useState(false);
  const [regKind, setRegKind] = useState<Api.SourceKind>('manual_doc');
  const [regTitle, setRegTitle] = useState('');
  const [regSourceRef, setRegSourceRef] = useState('');
  const [regContent, setRegContent] = useState('');
  const [registering, setRegistering] = useState(false);

  // Step 1b: Update source version dialog
  const [editTarget, setEditTarget] = useState<Api.KnowledgeSource | null>(
    null,
  );
  const [editTitle, setEditTitle] = useState('');
  const [editContent, setEditContent] = useState('');
  const [updatingSource, setUpdatingSource] = useState(false);

  const refreshData = async () => {
    try {
      const [kbRes, srcRes] = await Promise.all([
        assistantGetKnowledgeBase({id: knowledgeBaseId}),
        assistantListKnowledgeSources({id: knowledgeBaseId}),
      ]);
      if (kbRes) {
        setBase(kbRes);
      }
      if (Array.isArray(srcRes)) {
        setSources(srcRes);
      }
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '刷新知识库来源数据失败'));
    }
  };

  // Step 1: Register source only (does NOT auto-index or auto-publish)
  const handleRegisterSource = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const title = regTitle.trim();
    const content = regContent.trim();
    if (!title || !content) {
      return;
    }

    setRegistering(true);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      const created = await assistantRegisterKnowledgeSource(
        {id: knowledgeBaseId},
        {
          kind: regKind,
          title,
          sourceRef:
            regSourceRef.trim() ||
            (regKind === 'course_text' ? 'course_default' : 'doc_manual'),
          content,
        },
      );
      setRegisterOpen(false);
      setRegTitle('');
      setRegSourceRef('');
      setRegContent('');
      setRegKind('manual_doc');
      setStatusMsg(
        `已登记来源「${created.title ?? title}」（版本 v${created.sourceVersion ?? 1}）。请在列表中触发切片索引，就绪后再执行发布。`,
      );
      await refreshData();
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '登记新来源失败'));
    } finally {
      setRegistering(false);
    }
  };

  const handleOpenEditSource = (src: Api.KnowledgeSource) => {
    setEditTarget(src);
    setEditTitle(src.title ?? '');
    setEditContent(src.content ?? '');
    setErrorMsg(null);
  };

  // Step 1b: Update source content & bump sourceVersion (retains previously published version until new version is indexed & published)
  const handleUpdateSource = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!editTarget?.id) {
      return;
    }
    const content = editContent.trim();
    if (!content) {
      return;
    }

    setUpdatingSource(true);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      const updated = await assistantUpdateKnowledgeSource(
        {id: knowledgeBaseId, sourceId: editTarget.id},
        {
          title: editTitle.trim() || editTarget.title,
          content,
        },
      );
      setEditTarget(null);
      setStatusMsg(
        `来源「${updated.title ?? editTarget.title}」内容已升版至 v${updated.sourceVersion}。新版本尚未切片与发布前，原已发布版本（v${updated.publishedVersion ?? 0}）继续保持有效检索。`,
      );
      await refreshData();
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '更新来源内容升版失败'));
    } finally {
      setUpdatingSource(false);
    }
  };

  // Step 2: Trigger index job
  const handleTriggerIndex = async (src: Api.KnowledgeSource) => {
    if (!src.id) {
      return;
    }
    setBusySourceId(src.id);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      const job = await assistantStartIndexJob({id: src.id});
      setJobsBySource((prev) => ({...prev, [src.id!]: job}));
      if (job.state === 'paused_budget') {
        setErrorMsg(
          `来源「${src.title}」入库索引作业因后台日预算子额度不足已暂停（状态：${getIndexJobStateLabel(job.state)}）。请联系管理员调整后台入库子额度后重试。`,
        );
      } else {
        setStatusMsg(
          `来源「${src.title}」切片索引已完成（版本 v${job.sourceVersion ?? src.sourceVersion}，状态：${getIndexJobStateLabel(job.state)}）。现在可以点击「发布就绪版本」对外生效。`,
        );
      }
      await refreshData();
    } catch (err: unknown) {
      const pausedJob = extractBudgetPausedJob(err);
      if (pausedJob && src.id) {
        setJobsBySource((prev) => ({...prev, [src.id!]: pausedJob}));
        setErrorMsg(
          `来源「${src.title}」触发切片索引时遇到后台日预算上限，作业已置为暂停状态（paused_budget）。`,
        );
      } else {
        setErrorMsg(getErrorMessage(err, '触发切片索引作业失败'));
      }
      await refreshData();
    } finally {
      setBusySourceId(null);
    }
  };

  // Step 3: Publish ready version
  const handlePublishSource = async (src: Api.KnowledgeSource) => {
    if (!src.id) {
      return;
    }
    const targetVersion = src.readyVersion ?? src.sourceVersion ?? 1;
    setBusySourceId(src.id);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      await assistantPublishKnowledgeSource(
        {id: src.id},
        {expectedVersion: targetVersion},
      );
      setStatusMsg(
        `来源「${src.title}」版本 v${targetVersion} 已正式发布，学屿助手可在权限范围内检索并展示引用。`,
      );
      await refreshData();
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '发布就绪版本失败'));
    } finally {
      setBusySourceId(null);
    }
  };

  // Step 4: Revoke source immediately
  const handleRevokeSource = async (src: Api.KnowledgeSource) => {
    if (!src.id) {
      return;
    }
    setBusySourceId(src.id);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      await assistantRevokeKnowledgeSource({id: src.id});
      setStatusMsg(
        `来源「${src.title}」已立即撤回并清理派生索引切片，后续问答与历史引用均已阻断。`,
      );
      await refreshData();
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '撤回知识来源失败'));
    } finally {
      setBusySourceId(null);
    }
  };

  return (
    <Layout className="space-y-6">
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link href="/manage/knowledge">知识库管理</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>
              {base?.name || `知识库 #${knowledgeBaseId}`}
            </BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <Layout
        flow="stack"
        className="justify-between gap-4 sm:flex-row sm:items-start"
      >
        <Layout className="space-y-1.5">
          <Layout flow="row" className="flex-wrap items-center gap-2">
            <Heading level={1} size="page">
              {base?.name || '知识库来源与版本治理'}
            </Heading>
            {base ? (
              <>
                <Badge
                  variant={
                    base.visibility === 'public' ? 'default' : 'secondary'
                  }
                >
                  {getKnowledgeVisibilityLabel(base.visibility)}
                </Badge>
                <Badge variant="ghost">库版本 v{base.version ?? 1}</Badge>
              </>
            ) : null}
          </Layout>
          <Text as="p" className="text-sm text-muted-foreground">
            {base?.description ||
              '严格遵循「1. 登记或升版来源 → 2. 触发切片索引（受后台日预算约束） → 3. 发布就绪版本 → 4. 按需撤回清理」四步生命周期。'}
          </Text>
        </Layout>

        <Layout flow="row" className="flex-wrap items-center gap-2">
          <Button variant="secondary" asChild>
            <Link href="/manage/knowledge">返回列表</Link>
          </Button>
          <Dialog open={registerOpen} onOpenChange={setRegisterOpen}>
            <DialogTrigger asChild>
              <Button>
                <PlusIcon data-icon="inline-start" />
                登记新来源
              </Button>
            </DialogTrigger>
            <DialogContent className="sm:max-w-xl">
              <DialogHeader>
                <DialogTitle>登记新知识来源（第 1 步）</DialogTitle>
                <DialogDescription>
                  登记后来源处于「待切片索引」草稿状态，不会自动触发预算消耗或对外发布。
                </DialogDescription>
              </DialogHeader>
              <Layout
                as="form"
                onSubmit={(e) => void handleRegisterSource(e)}
                className="space-y-5"
              >
                <FieldGroup>
                  <Field>
                    <FieldLabel htmlFor="src-reg-kind">来源类型</FieldLabel>
                    <Select
                      value={regKind}
                      onValueChange={(val) => setRegKind(val as Api.SourceKind)}
                      disabled={registering}
                    >
                      <SelectTrigger id="src-reg-kind" className="w-full">
                        <SelectValue placeholder="选择来源类型" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectGroup>
                          <SelectItem value="manual_doc">
                            手动文档（manual_doc：通用学习指南、FAQ 或技术规范）
                          </SelectItem>
                          <SelectItem value="course_text">
                            授权课程图文（course_text：与特定课程 ID
                            绑定并校验学习权限）
                          </SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="src-reg-title">来源标题</FieldLabel>
                    <Input
                      id="src-reg-title"
                      placeholder="例如：第 1 章 微服务边界与接口契约规范"
                      value={regTitle}
                      onChange={(e) => setRegTitle(e.target.value)}
                      disabled={registering}
                      required
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="src-reg-ref">
                      引用标识（sourceRef）
                    </FieldLabel>
                    <Input
                      id="src-reg-ref"
                      placeholder={
                        regKind === 'course_text'
                          ? '填写关联的课程 ID（例如：course_go_01）'
                          : '填写文档标识（例如：doc_arch_v1）'
                      }
                      value={regSourceRef}
                      onChange={(e) => setRegSourceRef(e.target.value)}
                      disabled={registering}
                    />
                    <FieldDescription>
                      当类型为「授权课程图文」时，问答检索将校验提问者对该课程标识的访问权限。
                    </FieldDescription>
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="src-reg-content">正文内容</FieldLabel>
                    <Textarea
                      id="src-reg-content"
                      placeholder="输入结构化资料正文，切片作业将按自然段落与语义窗口切分..."
                      value={regContent}
                      onChange={(e) => setRegContent(e.target.value)}
                      disabled={registering}
                      rows={6}
                      required
                    />
                  </Field>
                </FieldGroup>
                <DialogFooter>
                  <Button
                    type="button"
                    variant="secondary"
                    onClick={() => setRegisterOpen(false)}
                    disabled={registering}
                  >
                    取消
                  </Button>
                  <Button
                    type="submit"
                    disabled={
                      registering || !regTitle.trim() || !regContent.trim()
                    }
                  >
                    {registering ? '登记中...' : '确认登记草稿'}
                  </Button>
                </DialogFooter>
              </Layout>
            </DialogContent>
          </Dialog>
        </Layout>
      </Layout>

      {errorMsg ? (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>风险与错误提示</AlertTitle>
          <AlertDescription>{errorMsg}</AlertDescription>
        </Alert>
      ) : null}

      {statusMsg ? (
        <Alert>
          <CheckCircle2Icon />
          <AlertTitle>状态更新</AlertTitle>
          <AlertDescription>{statusMsg}</AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-base">
            知识来源与版本状态列表 ({sources.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          {sources.length === 0 ? (
            <Empty className="py-10">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <FileTextIcon />
                </EmptyMedia>
                <EmptyTitle>当前知识库暂无来源</EmptyTitle>
                <EmptyDescription>
                  请点击右上角「登记新来源」录入文档或课程图文，再依次执行切片索引与版本发布。
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                <Button onClick={() => setRegisterOpen(true)}>
                  <PlusIcon data-icon="inline-start" />
                  登记新来源
                </Button>
              </EmptyContent>
            </Empty>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>来源标题与引用标识</TableHead>
                  <TableHead>来源类型</TableHead>
                  <TableHead>版本推进（当前 / 就绪 / 已发布）</TableHead>
                  <TableHead>状态与索引作业</TableHead>
                  <TableHead>更新时间</TableHead>
                  <TableHead className="text-right">分步治理操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sources.map((src) => {
                  const srcVer = src.sourceVersion ?? 1;
                  const readyVer = src.readyVersion ?? 0;
                  const pubVer = src.publishedVersion ?? 0;
                  const canPublish =
                    readyVer >= srcVer &&
                    readyVer > 0 &&
                    src.state !== 'revoked' &&
                    !(src.state === 'published' && pubVer === readyVer);
                  const isBusy = busySourceId === src.id;
                  const latestJob = src.id ? jobsBySource[src.id] : undefined;

                  return (
                    <TableRow key={src.id}>
                      <TableCell>
                        <Layout className="font-medium text-foreground">
                          {src.title || '未命名来源'}
                        </Layout>
                        <Layout className="mt-0.5 font-mono text-xs text-muted-foreground">
                          标识: {src.sourceRef || '—'}
                        </Layout>
                      </TableCell>
                      <TableCell>
                        <Badge variant="secondary">
                          {getSourceKindLabel(src.kind)}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Layout
                          flow="row"
                          className="flex-wrap items-center gap-1.5 text-xs"
                        >
                          <Badge variant="ghost">当前 v{srcVer}</Badge>
                          <Badge
                            variant={readyVer >= srcVer ? 'default' : 'ghost'}
                          >
                            就绪 v{readyVer}
                          </Badge>
                          <Badge variant={pubVer > 0 ? 'secondary' : 'ghost'}>
                            已发布 {pubVer > 0 ? `v${pubVer}` : '无'}
                          </Badge>
                        </Layout>
                        {pubVer > 0 && pubVer < srcVer ? (
                          <Layout className="mt-1 text-xs text-muted-foreground">
                            新版本发布前，线上仍检索 v{pubVer}
                          </Layout>
                        ) : null}
                      </TableCell>
                      <TableCell>
                        <Layout className="space-y-1">
                          <Badge
                            variant={
                              src.state === 'published'
                                ? 'default'
                                : src.state === 'revoked'
                                  ? 'destructive'
                                  : 'secondary'
                            }
                          >
                            {getSourceStateLabel(src.state)}
                          </Badge>
                          {latestJob ? (
                            <Layout className="text-xs text-muted-foreground">
                              作业: {getIndexJobStateLabel(latestJob.state)} ·{' '}
                              {getIndexJobStageLabel(latestJob.stage)}
                              {typeof latestJob.costCents === 'number'
                                ? `（耗时费用 ${formatCentsToYuan(latestJob.costCents)}）`
                                : ''}
                            </Layout>
                          ) : null}
                          {latestJob?.state === 'paused_budget' ? (
                            <Layout className="text-xs font-medium text-foreground">
                              提示:
                              后台入库子额度不足，作业已暂停（paused_budget）
                            </Layout>
                          ) : null}
                        </Layout>
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {formatDateTime(src.updatedAt ?? src.createdAt)}
                      </TableCell>
                      <TableCell className="text-right">
                        <Layout className="inline-flex flex-wrap items-center justify-end gap-1.5">
                          <Button
                            variant="secondary"
                            size="sm"
                            disabled={isBusy}
                            onClick={() => handleOpenEditSource(src)}
                          >
                            <PencilIcon data-icon="inline-start" />
                            更新内容升版
                          </Button>

                          <Button
                            variant="secondary"
                            size="sm"
                            disabled={isBusy}
                            onClick={() => void handleTriggerIndex(src)}
                          >
                            <PlayIcon data-icon="inline-start" />
                            {isBusy ? '处理中...' : '触发切片索引'}
                          </Button>

                          <AlertDialog>
                            <AlertDialogTrigger asChild>
                              <Button
                                size="sm"
                                disabled={!canPublish || isBusy}
                              >
                                <SendIcon data-icon="inline-start" />
                                发布就绪版本
                              </Button>
                            </AlertDialogTrigger>
                            <AlertDialogContent>
                              <AlertDialogHeader>
                                <AlertDialogTitle>
                                  确认发布就绪版本 v{readyVer}？
                                </AlertDialogTitle>
                                <AlertDialogDescription>
                                  来源「{src.title}」的就绪版本 v{readyVer}{' '}
                                  已完成切片索引。确认发布后，绑定该知识库的已发布
                                  Agent 将立即使用 v{readyVer}{' '}
                                  切片参与检索与引用回答。
                                </AlertDialogDescription>
                              </AlertDialogHeader>
                              <AlertDialogFooter>
                                <AlertDialogCancel>取消</AlertDialogCancel>
                                <AlertDialogAction
                                  onClick={() => void handlePublishSource(src)}
                                >
                                  确认发布 v{readyVer}
                                </AlertDialogAction>
                              </AlertDialogFooter>
                            </AlertDialogContent>
                          </AlertDialog>

                          <AlertDialog>
                            <AlertDialogTrigger asChild>
                              <Button
                                variant="destructive"
                                size="sm"
                                disabled={src.state === 'revoked' || isBusy}
                              >
                                <ShieldBanIcon data-icon="inline-start" />
                                撤回来源
                              </Button>
                            </AlertDialogTrigger>
                            <AlertDialogContent>
                              <AlertDialogHeader>
                                <AlertDialogTitle>
                                  立即撤回来源「{src.title}」？
                                </AlertDialogTitle>
                                <AlertDialogDescription>
                                  撤回后将立即停止该来源参与任何新的问答检索，清理派生索引切片，并在历史会话中隐藏或拦截已失效引用。此操作立即生效。
                                </AlertDialogDescription>
                              </AlertDialogHeader>
                              <AlertDialogFooter>
                                <AlertDialogCancel>取消</AlertDialogCancel>
                                <AlertDialogAction
                                  variant="destructive"
                                  onClick={() => void handleRevokeSource(src)}
                                >
                                  确认立即撤回
                                </AlertDialogAction>
                              </AlertDialogFooter>
                            </AlertDialogContent>
                          </AlertDialog>
                        </Layout>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Update Source Version Dialog */}
      <Dialog
        open={Boolean(editTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setEditTarget(null);
          }
        }}
      >
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>更新来源内容并升版</DialogTitle>
            <DialogDescription>
              当前版本为 v{editTarget?.sourceVersion ?? 1}
              （已发布版本：
              {(editTarget?.publishedVersion ?? 0) > 0
                ? `v${editTarget?.publishedVersion}`
                : '尚未发布'}
              ）。保存后版本号将递增为 v{(editTarget?.sourceVersion ?? 1) + 1}
              ；在新版本完成「触发切片索引」与「发布就绪版本」之前，原已发布版本继续对外有效。
            </DialogDescription>
          </DialogHeader>
          <Layout
            as="form"
            onSubmit={(e) => void handleUpdateSource(e)}
            className="space-y-5"
          >
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="src-edit-title">来源标题</FieldLabel>
                <Input
                  id="src-edit-title"
                  value={editTitle}
                  onChange={(e) => setEditTitle(e.target.value)}
                  disabled={updatingSource}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="src-edit-content">
                  新版本正文内容
                </FieldLabel>
                <Textarea
                  id="src-edit-content"
                  value={editContent}
                  onChange={(e) => setEditContent(e.target.value)}
                  disabled={updatingSource}
                  rows={8}
                  required
                />
                <FieldDescription>
                  保存升版后，请在列表中点击「触发切片索引」生成新版本切片。
                </FieldDescription>
              </Field>
            </FieldGroup>
            <DialogFooter>
              <Button
                type="button"
                variant="secondary"
                onClick={() => setEditTarget(null)}
                disabled={updatingSource}
              >
                取消
              </Button>
              <Button
                type="submit"
                disabled={updatingSource || !editContent.trim()}
              >
                {updatingSource ? '保存升版中...' : '保存并递增版本号'}
              </Button>
            </DialogFooter>
          </Layout>
        </DialogContent>
      </Dialog>
    </Layout>
  );
}
