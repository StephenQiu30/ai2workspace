'use client';

import {Layout} from '@/components/layout/layout';
import {Heading, Text} from '@/components/layout/typography';
import {useState} from 'react';
import Link from 'next/link';
import {
  AlertCircleIcon,
  CheckCircle2Icon,
  LockIcon,
  SaveIcon,
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
import {Checkbox} from '@/components/ui/checkbox';
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
import {Textarea} from '@/components/ui/textarea';
import {
  formatPricePerK,
  getAgentStateLabel,
  getKnowledgeVisibilityLabel,
  getProviderLabel,
  isModelExpired,
} from '@/features/assistant/assistant_helpers';
import {formatDateTime, getErrorMessage} from '@/lib/format';
import {
  assistantDisableAgent,
  assistantPublishAgent,
  assistantUpdateAgent,
} from '@/services/generated/assistant/agents';

export interface AgentConfigViewProps {
  agentParam: string;
  initialAgent: AssistantApi.Configuration | null;
  initialModels: AssistantApi.ModelInfo[];
  initialKnowledgeBases: AssistantApi.KnowledgeBase[];
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

export function AgentConfigView({
  agentParam,
  initialAgent,
  initialModels,
  initialKnowledgeBases,
  initialError,
}: AgentConfigViewProps) {
  const [agent, setAgent] = useState<AssistantApi.Configuration | null>(
    initialAgent,
  );
  const [models] = useState<AssistantApi.ModelInfo[]>(initialModels);
  const [knowledgeBases] = useState<AssistantApi.KnowledgeBase[]>(
    initialKnowledgeBases,
  );
  const [errorMsg, setErrorMsg] = useState<string | null>(initialError ?? null);
  const [statusMsg, setStatusMsg] = useState<string | null>(null);

  const initialModelKey = initialAgent
    ? buildModelKey(initialAgent.providerId, initialAgent.modelId)
    : models[0]
      ? buildModelKey(models[0].providerId, models[0].modelId)
      : 'deepseek::deepseek-chat';

  const [name, setName] = useState(initialAgent?.name ?? '');
  const [slug, setSlug] = useState(initialAgent?.slug ?? '');
  const [roleInstruction, setRoleInstruction] = useState(
    initialAgent?.roleInstruction ?? '',
  );
  const [modelKey, setModelKey] = useState(initialModelKey);
  const [selectedKbIds, setSelectedKbIds] = useState<string[]>(
    initialAgent?.knowledgeBaseIds ?? [],
  );
  const [disableReason, setDisableReason] = useState(
    initialAgent?.disabledReason ?? '管理员在独立配置页停用该实例',
  );

  const [saving, setSaving] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [disabling, setDisabling] = useState(false);

  const toggleKnowledgeBase = (kbId: string, checked: boolean) => {
    setSelectedKbIds((prev) => {
      if (checked) {
        return prev.includes(kbId) ? prev : [...prev, kbId];
      }
      return prev.filter((id) => id !== kbId);
    });
  };

  const applyUpdatedAgent = (updated: AssistantApi.Configuration) => {
    setAgent(updated);
    setName(updated.name ?? '');
    setSlug(updated.slug ?? '');
    setRoleInstruction(updated.roleInstruction ?? '');
    if (updated.providerId && updated.modelId) {
      setModelKey(buildModelKey(updated.providerId, updated.modelId));
    }
    setSelectedKbIds(updated.knowledgeBaseIds ?? []);
  };

  const handleSaveDraft = async (e?: React.FormEvent<HTMLFormElement>) => {
    if (e) {
      e.preventDefault();
    }
    if (!agent?.id) {
      setErrorMsg('未找到有效的 Agent ID，无法保存草稿配置');
      return;
    }
    const trimmedName = name.trim();
    const trimmedInstruction = roleInstruction.trim();
    if (!trimmedName || !trimmedInstruction) {
      setErrorMsg('实例名称与角色提示词不能为空');
      return;
    }

    const {providerId, modelId} = parseModelKey(modelKey);
    setSaving(true);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      const updated = await assistantUpdateAgent(
        {id: agent.id},
        {
          name: trimmedName,
          slug: slug.trim() || agent.slug,
          roleInstruction: trimmedInstruction,
          providerId,
          modelId,
          knowledgeBaseIds: selectedKbIds,
          expectedVersion: agent.draftVersion,
        },
      );
      applyUpdatedAgent(updated);
      setStatusMsg(
        `草稿配置已保存，草稿版本已递增至 v${updated.draftVersion}（当前线上发布版本：${
          (updated.publishedVersion ?? 0) > 0
            ? `v${updated.publishedVersion}`
            : '尚未发布'
        }）。如需对外生效，请点击「发布当前版本」。`,
      );
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '保存草稿配置失败'));
    } finally {
      setSaving(false);
    }
  };

  const handlePublish = async () => {
    if (!agent?.id) {
      return;
    }
    setPublishing(true);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      const published = await assistantPublishAgent({id: agent.id});
      applyUpdatedAgent(published);
      setStatusMsg(
        `Agent「${published.name}」版本 v${published.publishedVersion} 已正式发布并对外生效。`,
      );
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '发布当前版本失败'));
    } finally {
      setPublishing(false);
    }
  };

  const handleDisable = async () => {
    if (!agent?.id) {
      return;
    }
    setDisabling(true);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      const disabled = await assistantDisableAgent(
        {id: agent.id},
        {reason: disableReason.trim() || '管理员执行停用'},
      );
      applyUpdatedAgent(disabled);
      setStatusMsg(
        `Agent「${disabled.name}」已停用，任何发往该实例的新问答请求均将被拒绝。`,
      );
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '停用实例失败'));
    } finally {
      setDisabling(false);
    }
  };

  return (
    <Layout className="space-y-6">
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link href="/manage/agents">Agent 管理</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>{agent?.name || agentParam}</BreadcrumbPage>
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
              {agent?.name ? `配置 Agent：${agent.name}` : 'Agent 实例详细配置'}
            </Heading>
            {agent ? (
              <>
                <Badge
                  variant={
                    agent.state === 'published'
                      ? 'default'
                      : agent.state === 'disabled'
                        ? 'destructive'
                        : 'secondary'
                  }
                >
                  {getAgentStateLabel(agent.state)}
                </Badge>
                <Badge variant="ghost">
                  草稿版本 v{agent.draftVersion ?? 1}
                </Badge>
                <Badge variant="secondary">
                  发布版本{' '}
                  {(agent.publishedVersion ?? 0) > 0
                    ? `v${agent.publishedVersion}`
                    : '未发布'}
                </Badge>
              </>
            ) : null}
          </Layout>
          <Text as="p" className="text-sm text-muted-foreground">
            调整角色提示词、受控模型绑定与多知识库检索授权。保存草稿不会影响线上已发布版本，确认无误后再执行发布。
          </Text>
        </Layout>

        <Layout flow="row" className="flex-wrap items-center gap-2">
          <Button variant="secondary" asChild>
            <Link href="/manage/agents">返回列表</Link>
          </Button>
          <Button
            variant="secondary"
            disabled={!agent?.id || saving || publishing || disabling}
            onClick={() => void handleSaveDraft()}
          >
            <SaveIcon data-icon="inline-start" />
            {saving ? '保存中...' : '保存草稿配置'}
          </Button>

          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button
                disabled={!agent?.id || saving || publishing || disabling}
              >
                <SendIcon data-icon="inline-start" />
                {publishing ? '发布中...' : '发布当前版本'}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>
                  确认发布「{agent?.name}」版本 v{agent?.draftVersion ?? 1}？
                </AlertDialogTitle>
                <AlertDialogDescription>
                  发布前请确保已点击「保存草稿配置」。发布后，已保存的草稿版本 v
                  {agent?.draftVersion ?? 1} 将立即成为线上运行版本，绑定{' '}
                  {selectedKbIds.length} 个授权知识库参与检索。
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>取消</AlertDialogCancel>
                <AlertDialogAction onClick={() => void handlePublish()}>
                  确认发布版本 v{agent?.draftVersion ?? 1}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>

          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button
                variant="destructive"
                disabled={
                  !agent?.id ||
                  agent.state === 'disabled' ||
                  saving ||
                  publishing ||
                  disabling
                }
              >
                <ShieldBanIcon data-icon="inline-start" />
                {disabling ? '停用中...' : '停用实例'}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>
                  确认停用实例「{agent?.name}」？
                </AlertDialogTitle>
                <AlertDialogDescription>
                  停用后，该 Agent
                  实例将立即停止服务，普通用户无法通过该实例发起任何新的问答请求。
                </AlertDialogDescription>
              </AlertDialogHeader>
              <Field className="py-2">
                <FieldLabel htmlFor="agent-disable-reason">停用原因</FieldLabel>
                <Input
                  id="agent-disable-reason"
                  value={disableReason}
                  onChange={(e) => setDisableReason(e.target.value)}
                  placeholder="输入停用说明..."
                />
              </Field>
              <AlertDialogFooter>
                <AlertDialogCancel>取消</AlertDialogCancel>
                <AlertDialogAction
                  variant="destructive"
                  onClick={() => void handleDisable()}
                >
                  确认停用实例
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </Layout>
      </Layout>

      {errorMsg ? (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>配置提示</AlertTitle>
          <AlertDescription>{errorMsg}</AlertDescription>
        </Alert>
      ) : null}

      {statusMsg ? (
        <Alert>
          <CheckCircle2Icon />
          <AlertTitle>操作成功</AlertTitle>
          <AlertDescription>{statusMsg}</AlertDescription>
        </Alert>
      ) : null}

      {/* Mandatory Read-only Governance Policy Notice */}
      <Alert>
        <LockIcon />
        <AlertTitle>已启用每答必联网搜索与 AI 预算准入（不可关闭）</AlertTitle>
        <AlertDescription>
          平台强制治理策略：所有 Agent 实例在每次回答前均强制经过「四层 AI
          预算与频次准入校验（预占额度）」，并执行「实时联网搜索 +
          授权知识库检索」双重证据汇合；管理员与讲师均不可关闭此安全策略。
        </AlertDescription>
      </Alert>

      {agent ? (
        <Layout
          as="form"
          onSubmit={(e) => void handleSaveDraft(e)}
          className="space-y-6"
        >
          <Card>
            <CardHeader>
              <CardTitle className="text-base">
                基础标识与受控模型配置
              </CardTitle>
            </CardHeader>
            <CardContent>
              <FieldGroup>
                <Layout
                  flow="grid"
                  className="grid-cols-1 gap-5 sm:grid-cols-2"
                >
                  <Field>
                    <FieldLabel htmlFor="cfg-agent-name">实例名称</FieldLabel>
                    <Input
                      id="cfg-agent-name"
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      disabled={saving}
                      required
                    />
                    <FieldDescription>
                      向学员展示在学屿助手顶部的实例名称。
                    </FieldDescription>
                  </Field>

                  <Field>
                    <FieldLabel htmlFor="cfg-agent-slug">
                      唯一标识（Slug）
                    </FieldLabel>
                    <Input
                      id="cfg-agent-slug"
                      value={slug}
                      onChange={(e) => setSlug(e.target.value)}
                      disabled={saving}
                      required
                    />
                    <FieldDescription>
                      系统调用与会话绑定的唯一标识符（如
                      customer_support、course_tutor）。
                    </FieldDescription>
                  </Field>
                </Layout>

                <Field>
                  <FieldLabel htmlFor="cfg-agent-model">
                    允许模型（从四家受控模型目录选择）
                  </FieldLabel>
                  <Select
                    value={modelKey}
                    onValueChange={setModelKey}
                    disabled={saving}
                  >
                    <SelectTrigger id="cfg-agent-model" className="w-full">
                      <SelectValue placeholder="选择允许的模型" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectGroup>
                        {models.map((m) => {
                          const key = buildModelKey(m.providerId, m.modelId);
                          const expired = isModelExpired(m.expiresAt);
                          return (
                            <SelectItem key={key} value={key}>
                              {getProviderLabel(m.providerId)} ·{' '}
                              {m.displayName || m.modelId} ({m.providerId}:
                              {m.modelId}) · 输入{' '}
                              {formatPricePerK(m.promptPricePerK, m.currency)} /
                              输出{' '}
                              {formatPricePerK(
                                m.completionPricePerK,
                                m.currency,
                              )}
                              {m.enabled === false
                                ? ' [已停用]'
                                : expired
                                  ? ' [价目已过期]'
                                  : ''}
                            </SelectItem>
                          );
                        })}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FieldDescription>
                    仅允许绑定 DeepSeek、OpenRouter、火山引擎、OpenAI
                    四家受控目录内且价目有效的模型。
                  </FieldDescription>
                </Field>

                <Field>
                  <FieldLabel htmlFor="cfg-agent-instruction">
                    角色提示词（Role Instruction）
                  </FieldLabel>
                  <Textarea
                    id="cfg-agent-instruction"
                    value={roleInstruction}
                    onChange={(e) => setRoleInstruction(e.target.value)}
                    disabled={saving}
                    rows={6}
                    required
                  />
                  <FieldDescription>
                    服务端将自动把该角色提示词与固定安全头组合；内部
                    systemPrompt 原文不会暴露给前端普通用户接口。
                  </FieldDescription>
                </Field>
              </FieldGroup>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <Layout>
                <CardTitle className="text-base">
                  多选授权知识库（已选 {selectedKbIds.length} /{' '}
                  {knowledgeBases.length}）
                </CardTitle>
                <Text as="p" className="mt-1 text-xs text-muted-foreground">
                  可同时勾选多个授权知识库。问答检索时仅会召回已勾选知识库内处于「已发布」状态且提问者具备课程访问权限的切片。
                </Text>
              </Layout>
              <Layout flow="row" className="items-center gap-2">
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() =>
                    setSelectedKbIds(
                      knowledgeBases
                        .map((kb) => kb.id)
                        .filter((id): id is string => Boolean(id)),
                    )
                  }
                  disabled={saving || knowledgeBases.length === 0}
                >
                  全选
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => setSelectedKbIds([])}
                  disabled={saving || selectedKbIds.length === 0}
                >
                  清空
                </Button>
              </Layout>
            </CardHeader>
            <CardContent>
              {knowledgeBases.length === 0 ? (
                <Card
                  surface="muted"
                  className="p-6 text-center text-sm text-muted-foreground block"
                >
                  当前尚未创建任何知识库，请先前往{' '}
                  <Link
                    href="/manage/knowledge"
                    className="font-medium text-foreground underline underline-offset-4"
                  >
                    知识库管理
                  </Link>{' '}
                  创建并发布来源。
                </Card>
              ) : (
                <Layout
                  flow="grid"
                  className="grid-cols-1 gap-3 sm:grid-cols-2"
                >
                  {knowledgeBases.map((kb) => {
                    if (!kb.id) {
                      return null;
                    }
                    const checked = selectedKbIds.includes(kb.id);
                    return (
                      <FieldLabel
                        key={kb.id}
                        className="flex cursor-pointer items-start justify-between gap-3 rounded-lg bg-canvas-soft p-3.5 transition-colors hover:bg-secondary"
                      >
                        <Layout flow="row" className="items-start gap-3">
                          <Checkbox
                            checked={checked}
                            onCheckedChange={(val) =>
                              toggleKnowledgeBase(kb.id!, val === true)
                            }
                            disabled={saving}
                            className="mt-0.5"
                          />
                          <Layout className="space-y-1">
                            <Layout className="text-sm font-medium text-foreground">
                              {kb.name}
                            </Layout>
                            {kb.description ? (
                              <Layout className="line-clamp-2 text-xs text-muted-foreground">
                                {kb.description}
                              </Layout>
                            ) : null}
                            <Layout className="font-mono text-[11px] text-muted-foreground">
                              ID: {kb.id} · 更新于{' '}
                              {formatDateTime(kb.updatedAt ?? kb.createdAt)}
                            </Layout>
                          </Layout>
                        </Layout>
                        <Badge
                          variant={
                            kb.visibility === 'public' ? 'default' : 'secondary'
                          }
                        >
                          {getKnowledgeVisibilityLabel(kb.visibility)}
                        </Badge>
                      </FieldLabel>
                    );
                  })}
                </Layout>
              )}
            </CardContent>
          </Card>

          <Layout
            flow="row"
            className="flex-wrap items-center justify-end gap-3"
          >
            <Button variant="secondary" asChild>
              <Link href="/manage/agents">返回 Agent 列表</Link>
            </Button>
            <Button
              type="submit"
              disabled={saving || !name.trim() || !roleInstruction.trim()}
            >
              <SaveIcon data-icon="inline-start" />
              {saving
                ? '保存草稿中...'
                : `保存草稿配置（当前 v${agent.draftVersion ?? 1}）`}
            </Button>
          </Layout>
        </Layout>
      ) : null}
    </Layout>
  );
}
