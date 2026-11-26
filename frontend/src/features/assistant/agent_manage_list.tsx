'use client';

import {Layout} from '@/components/layout/layout';
import {Heading, Text} from '@/components/layout/typography';
import {useState} from 'react';
import Link from 'next/link';
import {
  AlertCircleIcon,
  BotIcon,
  CheckCircle2Icon,
  PlusIcon,
  SendIcon,
  Settings2Icon,
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
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Checkbox} from '@/components/ui/checkbox';
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
  getAgentStateLabel,
  getProviderLabel,
  isModelExpired,
} from '@/features/assistant/assistant_helpers';
import {formatDateTime, getErrorMessage} from '@/lib/format';
import {
  assistantCreateAgent,
  assistantDisableAgent,
  assistantListAgents,
  assistantPublishAgent,
} from '@/services/generated/api/agents';

export interface AgentManageListProps {
  initialAgents: Api.Configuration[];
  initialModels: Api.ModelInfo[];
  initialKnowledgeBases: Api.KnowledgeBase[];
  initialError?: string | null;
}

function buildModelKey(providerId?: string, modelId?: string): string {
  return `${providerId ?? ''}::${modelId ?? ''}`;
}

function parseModelKey(key: string): {providerId: string; modelId: string} {
  const idx = key.indexOf('::');
  if (idx === -1) {
    return {providerId: 'deepseek', modelId: key};
  }
  return {
    providerId: key.slice(0, idx),
    modelId: key.slice(idx + 2),
  };
}

export function AgentManageList({
  initialAgents,
  initialModels,
  initialKnowledgeBases,
  initialError,
}: AgentManageListProps) {
  const [agents, setAgents] = useState<Api.Configuration[]>(initialAgents);
  const [models] = useState<Api.ModelInfo[]>(initialModels);
  const [knowledgeBases] = useState<Api.KnowledgeBase[]>(initialKnowledgeBases);
  const [errorMsg, setErrorMsg] = useState<string | null>(initialError ?? null);
  const [statusMsg, setStatusMsg] = useState<string | null>(null);
  const [busyAgentId, setBusyAgentId] = useState<string | null>(null);

  const defaultModel =
    models.find((m) => m.enabled !== false && !isModelExpired(m.expiresAt)) ??
    models[0];
  const defaultModelKey = defaultModel
    ? buildModelKey(defaultModel.providerId, defaultModel.modelId)
    : 'deepseek::deepseek-chat';

  // Create Agent Draft Dialog State
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createSlug, setCreateSlug] = useState('');
  const [createRoleInstruction, setCreateRoleInstruction] = useState('');
  const [createModelKey, setCreateModelKey] = useState(defaultModelKey);
  const [createKbIds, setCreateKbIds] = useState<string[]>([]);
  const [creating, setCreating] = useState(false);

  const refreshAgents = async () => {
    try {
      const list = await assistantListAgents();
      if (Array.isArray(list)) {
        setAgents(list);
      }
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '刷新 Agent 实例列表失败'));
    }
  };

  const toggleCreateKb = (kbId: string, checked: boolean) => {
    setCreateKbIds((prev) => {
      if (checked) {
        return prev.includes(kbId) ? prev : [...prev, kbId];
      }
      return prev.filter((id) => id !== kbId);
    });
  };

  // Creates draft only (does NOT auto-publish)
  const handleCreateDraft = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const name = createName.trim();
    const slug = createSlug.trim();
    const roleInstruction = createRoleInstruction.trim();
    if (!name || !slug || !roleInstruction) {
      return;
    }

    const {providerId, modelId} = parseModelKey(createModelKey);
    setCreating(true);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      const created = await assistantCreateAgent({
        name,
        slug,
        roleInstruction,
        providerId,
        modelId,
        knowledgeBaseIds: createKbIds,
      });
      setCreateOpen(false);
      setCreateName('');
      setCreateSlug('');
      setCreateRoleInstruction('');
      setCreateKbIds([]);
      setStatusMsg(
        `已创建 Agent 草稿「${created.name ?? name}」（草稿版本 v${created.draftVersion ?? 1}）。可点击「进入配置」继续调优，或直接发布上线。`,
      );
      await refreshAgents();
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '创建 Agent 草稿失败'));
    } finally {
      setCreating(false);
    }
  };

  const handlePublish = async (agent: Api.Configuration) => {
    if (!agent.id) {
      return;
    }
    setBusyAgentId(agent.id);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      const published = await assistantPublishAgent({id: agent.id});
      setStatusMsg(
        `Agent 实例「${published.name ?? agent.name}」已发布版本 v${published.publishedVersion ?? agent.draftVersion}，前端学屿助手现可选择并调用。`,
      );
      await refreshAgents();
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '发布 Agent 实例失败'));
    } finally {
      setBusyAgentId(null);
    }
  };

  const handleDisable = async (agent: Api.Configuration) => {
    if (!agent.id) {
      return;
    }
    setBusyAgentId(agent.id);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      const disabled = await assistantDisableAgent(
        {id: agent.id},
        {reason: '管理员在控制台执行停用处置'},
      );
      setStatusMsg(
        `Agent 实例「${disabled.name ?? agent.name}」已停用，新的问答请求将被拒绝。`,
      );
      await refreshAgents();
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '停用 Agent 实例失败'));
    } finally {
      setBusyAgentId(null);
    }
  };

  return (
    <Layout className="space-y-6">
      <Layout
        flow="stack"
        className="justify-between gap-4 sm:flex-row sm:items-center"
      >
        <Layout>
          <Heading level={1} size="page">
            Agent 实例与配置管理
          </Heading>
          <Text as="p" className="mt-1 text-sm text-muted-foreground">
            仅平台管理员可创建、配置、发布或停用学屿助手 Agent
            实例（R-28），统一绑定允许模型与多知识库授权。
          </Text>
        </Layout>

        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogTrigger asChild>
            <Button>
              <PlusIcon data-icon="inline-start" />
              新建 Agent
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-xl">
            <DialogHeader>
              <DialogTitle>新建 Agent 草稿配置</DialogTitle>
              <DialogDescription>
                创建后默认处于「草稿」状态，不会自动对外发布。您可在独立配置页核对角色提示词与多知识库授权后再发布。
              </DialogDescription>
            </DialogHeader>
            <Layout
              as="form"
              onSubmit={(e) => void handleCreateDraft(e)}
              className="space-y-5"
            >
              <FieldGroup>
                <Layout
                  flow="grid"
                  className="grid-cols-1 gap-4 sm:grid-cols-2"
                >
                  <Field>
                    <FieldLabel htmlFor="agent-create-name">
                      实例名称
                    </FieldLabel>
                    <Input
                      id="agent-create-name"
                      placeholder="例如：课程学习辅导助教"
                      value={createName}
                      onChange={(e) => setCreateName(e.target.value)}
                      disabled={creating}
                      required
                    />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="agent-create-slug">
                      唯一标识（Slug）
                    </FieldLabel>
                    <Input
                      id="agent-create-slug"
                      placeholder="例如：course_tutor"
                      value={createSlug}
                      onChange={(e) => setCreateSlug(e.target.value)}
                      disabled={creating}
                      required
                    />
                  </Field>
                </Layout>

                <Field>
                  <FieldLabel htmlFor="agent-create-model">
                    绑定允许模型（四家提供商受控目录）
                  </FieldLabel>
                  <Select
                    value={createModelKey}
                    onValueChange={setCreateModelKey}
                    disabled={creating}
                  >
                    <SelectTrigger id="agent-create-model" className="w-full">
                      <SelectValue placeholder="选择提供商与模型" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectGroup>
                        {models.length > 0 ? (
                          models.map((m) => {
                            const key = buildModelKey(m.providerId, m.modelId);
                            const expired = isModelExpired(m.expiresAt);
                            return (
                              <SelectItem key={key} value={key}>
                                {getProviderLabel(m.providerId)} ·{' '}
                                {m.displayName || m.modelId} ({m.providerId}:
                                {m.modelId})
                                {m.enabled === false
                                  ? ' [已停用]'
                                  : expired
                                    ? ' [价目已过期]'
                                    : ''}
                              </SelectItem>
                            );
                          })
                        ) : (
                          <SelectItem value="deepseek::deepseek-chat">
                            DeepSeek · deepseek-chat (deepseek:deepseek-chat)
                          </SelectItem>
                        )}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </Field>

                <Field>
                  <FieldLabel>
                    多选授权知识库（{createKbIds.length} 个已选）
                  </FieldLabel>
                  {knowledgeBases.length === 0 ? (
                    <FieldDescription>
                      当前暂无可选知识库，可先创建草稿后再前往「知识库管理」新建。
                    </FieldDescription>
                  ) : (
                    <Layout className="max-h-40 space-y-2 overflow-y-auto rounded-md bg-canvas-soft p-3">
                      {knowledgeBases.map((kb) => {
                        if (!kb.id) {
                          return null;
                        }
                        const checked = createKbIds.includes(kb.id);
                        return (
                          <FieldLabel
                            key={kb.id}
                            className="flex cursor-pointer items-center justify-between gap-3 rounded-md px-2 py-1.5 text-sm hover:bg-secondary"
                          >
                            <Text className="flex items-center gap-2.5">
                              <Checkbox
                                checked={checked}
                                onCheckedChange={(val) =>
                                  toggleCreateKb(kb.id!, val === true)
                                }
                                disabled={creating}
                              />
                              <Text className="font-medium text-foreground">
                                {kb.name}
                              </Text>
                            </Text>
                            <Badge variant="ghost" className="text-xs">
                              {kb.visibility === 'public' ? '公开' : '私有'}
                            </Badge>
                          </FieldLabel>
                        );
                      })}
                    </Layout>
                  )}
                </Field>

                <Field>
                  <FieldLabel htmlFor="agent-create-prompt">
                    角色指导提示词（Role Instruction）
                  </FieldLabel>
                  <Textarea
                    id="agent-create-prompt"
                    placeholder="定义该 Agent 的专业角色边界、回答结构规范与引用约束（不向前端匿名或普通学员暴露内部原文）..."
                    value={createRoleInstruction}
                    onChange={(e) => setCreateRoleInstruction(e.target.value)}
                    disabled={creating}
                    rows={4}
                    required
                  />
                  <FieldDescription>
                    只读策略：所有 Agent
                    实例默认强制开启「每答必联网搜索」与「AI
                    预算准入校验」，不可关闭。
                  </FieldDescription>
                </Field>
              </FieldGroup>
              <DialogFooter>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => setCreateOpen(false)}
                  disabled={creating}
                >
                  取消
                </Button>
                <Button
                  type="submit"
                  disabled={
                    creating ||
                    !createName.trim() ||
                    !createSlug.trim() ||
                    !createRoleInstruction.trim()
                  }
                >
                  {creating ? '保存草稿中...' : '创建草稿'}
                </Button>
              </DialogFooter>
            </Layout>
          </DialogContent>
        </Dialog>
      </Layout>

      {errorMsg ? (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>操作提示</AlertTitle>
          <AlertDescription>{errorMsg}</AlertDescription>
        </Alert>
      ) : null}

      {statusMsg ? (
        <Alert>
          <CheckCircle2Icon />
          <AlertTitle>状态已更新</AlertTitle>
          <AlertDescription>{statusMsg}</AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-base">
            全部 Agent 实例 ({agents.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          {agents.length === 0 ? (
            <Empty className="py-10">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <BotIcon />
                </EmptyMedia>
                <EmptyTitle>暂无 Agent 实例</EmptyTitle>
                <EmptyDescription>
                  点击右上角「新建
                  Agent」创建草稿并配置角色提示词、绑定模型与授权知识库。
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                <Button onClick={() => setCreateOpen(true)}>
                  <PlusIcon data-icon="inline-start" />
                  新建 Agent
                </Button>
              </EmptyContent>
            </Empty>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>实例名称</TableHead>
                  <TableHead>唯一标识（Slug）</TableHead>
                  <TableHead>运行状态</TableHead>
                  <TableHead>绑定模型</TableHead>
                  <TableHead>绑定知识库</TableHead>
                  <TableHead>草稿 / 发布版本</TableHead>
                  <TableHead>更新时间</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {agents.map((ag) => {
                  const isBusy = busyAgentId === ag.id;
                  const kbCount = ag.knowledgeBaseIds?.length ?? 0;
                  const configIdentifier = ag.slug || ag.id || '';

                  return (
                    <TableRow key={ag.id ?? ag.slug}>
                      <TableCell>
                        <Layout className="font-medium text-foreground">
                          {ag.name || '未命名实例'}
                        </Layout>
                        {ag.disabledReason && ag.state === 'disabled' ? (
                          <Layout className="mt-0.5 text-xs text-muted-foreground">
                            停用原因: {ag.disabledReason}
                          </Layout>
                        ) : null}
                      </TableCell>
                      <TableCell className="font-mono text-xs text-muted-foreground">
                        {ag.slug}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={
                            ag.state === 'published'
                              ? 'default'
                              : ag.state === 'disabled'
                                ? 'destructive'
                                : 'secondary'
                          }
                        >
                          {getAgentStateLabel(ag.state)}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant="ghost" className="font-mono text-xs">
                          {ag.providerId}:{ag.modelId}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Text className="text-sm text-foreground">
                          {kbCount} 个知识库
                        </Text>
                      </TableCell>
                      <TableCell>
                        <Layout
                          flow="row"
                          className="items-center gap-1.5 text-xs"
                        >
                          <Badge variant="ghost">
                            草稿 v{ag.draftVersion ?? 1}
                          </Badge>
                          <Badge
                            variant={
                              (ag.publishedVersion ?? 0) > 0
                                ? 'secondary'
                                : 'outline'
                            }
                          >
                            发布{' '}
                            {(ag.publishedVersion ?? 0) > 0
                              ? `v${ag.publishedVersion}`
                              : '未发布'}
                          </Badge>
                        </Layout>
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {formatDateTime(ag.updatedAt ?? ag.createdAt)}
                      </TableCell>
                      <TableCell className="text-right">
                        <Layout className="inline-flex flex-wrap items-center justify-end gap-1.5">
                          {configIdentifier ? (
                            <Button variant="secondary" size="sm" asChild>
                              <Link
                                href={`/manage/agents/${encodeURIComponent(configIdentifier)}`}
                              >
                                <Settings2Icon data-icon="inline-start" />
                                进入配置
                              </Link>
                            </Button>
                          ) : null}

                          <AlertDialog>
                            <AlertDialogTrigger asChild>
                              <Button size="sm" disabled={isBusy || !ag.id}>
                                <SendIcon data-icon="inline-start" />
                                发布
                              </Button>
                            </AlertDialogTrigger>
                            <AlertDialogContent>
                              <AlertDialogHeader>
                                <AlertDialogTitle>
                                  确认发布 Agent「{ag.name}」？
                                </AlertDialogTitle>
                                <AlertDialogDescription>
                                  将草稿版本 v{ag.draftVersion ?? 1}{' '}
                                  正式发布为线上生效版本（绑定模型{' '}
                                  {ag.providerId}:{ag.modelId}，授权知识库{' '}
                                  {kbCount}{' '}
                                  个）。发布后普通学员可在学屿助手中选择该实例发起提问。
                                </AlertDialogDescription>
                              </AlertDialogHeader>
                              <AlertDialogFooter>
                                <AlertDialogCancel>取消</AlertDialogCancel>
                                <AlertDialogAction
                                  onClick={() => void handlePublish(ag)}
                                >
                                  确认发布 v{ag.draftVersion ?? 1}
                                </AlertDialogAction>
                              </AlertDialogFooter>
                            </AlertDialogContent>
                          </AlertDialog>

                          <AlertDialog>
                            <AlertDialogTrigger asChild>
                              <Button
                                variant="destructive"
                                size="sm"
                                disabled={
                                  isBusy || !ag.id || ag.state === 'disabled'
                                }
                              >
                                <ShieldBanIcon data-icon="inline-start" />
                                停用
                              </Button>
                            </AlertDialogTrigger>
                            <AlertDialogContent>
                              <AlertDialogHeader>
                                <AlertDialogTitle>
                                  确认停用 Agent「{ag.name}」？
                                </AlertDialogTitle>
                                <AlertDialogDescription>
                                  停用后该实例将立即从普通用户可选列表下线，并拒绝任何发往该实例的新问答请求。您可随时在配置页重新发布恢复。
                                </AlertDialogDescription>
                              </AlertDialogHeader>
                              <AlertDialogFooter>
                                <AlertDialogCancel>取消</AlertDialogCancel>
                                <AlertDialogAction
                                  variant="destructive"
                                  onClick={() => void handleDisable(ag)}
                                >
                                  确认停用实例
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
    </Layout>
  );
}
