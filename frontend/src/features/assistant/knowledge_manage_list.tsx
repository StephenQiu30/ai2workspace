'use client';

import {Layout} from '@/components/layout/layout';
import {Heading, Text} from '@/components/layout/typography';
import {useState} from 'react';
import Link from 'next/link';
import {
  AlertCircleIcon,
  CheckCircle2Icon,
  FolderOpenIcon,
  PencilIcon,
  PlusIcon,
} from 'lucide-react';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {Badge} from '@/components/ui/badge';
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
import {getKnowledgeVisibilityLabel} from '@/features/assistant/assistant_helpers';
import {formatDateTime, getErrorMessage} from '@/lib/format';
import {
  assistantCreateKnowledgeBase,
  assistantListKnowledgeBases,
  assistantUpdateKnowledgeBase,
} from '@/services/generated/assistant/knowledge';

export interface KnowledgeManageListProps {
  initialBases: AssistantApi.KnowledgeBase[];
  initialError?: string | null;
}

export function KnowledgeManageList({
  initialBases,
  initialError,
}: KnowledgeManageListProps) {
  const [bases, setBases] =
    useState<AssistantApi.KnowledgeBase[]>(initialBases);
  const [errorMsg, setErrorMsg] = useState<string | null>(initialError ?? null);
  const [statusMsg, setStatusMsg] = useState<string | null>(null);

  // Create Dialog State
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createDescription, setCreateDescription] = useState('');
  const [createVisibility, setCreateVisibility] =
    useState<AssistantApi.Visibility>('public');
  const [creating, setCreating] = useState(false);

  // Edit Dialog State
  const [editTarget, setEditTarget] =
    useState<AssistantApi.KnowledgeBase | null>(null);
  const [editName, setEditName] = useState('');
  const [editDescription, setEditDescription] = useState('');
  const [editVisibility, setEditVisibility] =
    useState<AssistantApi.Visibility>('public');
  const [updating, setUpdating] = useState(false);

  const refreshBases = async () => {
    try {
      const list = await assistantListKnowledgeBases();
      if (Array.isArray(list)) {
        setBases(list);
      }
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '刷新知识库列表失败'));
    }
  };

  const handleOpenEdit = (kb: AssistantApi.KnowledgeBase) => {
    setEditTarget(kb);
    setEditName(kb.name ?? '');
    setEditDescription(kb.description ?? '');
    setEditVisibility(kb.visibility ?? 'public');
    setErrorMsg(null);
  };

  const handleCreate = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const trimmedName = createName.trim();
    if (!trimmedName) {
      return;
    }

    setCreating(true);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      const created = await assistantCreateKnowledgeBase({
        name: trimmedName,
        description: createDescription.trim() || undefined,
        visibility: createVisibility,
      });
      setCreateOpen(false);
      setCreateName('');
      setCreateDescription('');
      setCreateVisibility('public');
      setStatusMsg(`已新建知识库「${created.name ?? trimmedName}」`);
      await refreshBases();
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '新建知识库失败'));
    } finally {
      setCreating(false);
    }
  };

  const handleUpdate = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!editTarget?.id) {
      return;
    }
    const trimmedName = editName.trim();
    if (!trimmedName) {
      return;
    }

    setUpdating(true);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      const updated = await assistantUpdateKnowledgeBase(
        {id: editTarget.id},
        {
          name: trimmedName,
          description: editDescription.trim(),
          visibility: editVisibility,
          expectedVersion: editTarget.version,
        },
      );
      setEditTarget(null);
      setStatusMsg(`已更新知识库「${updated.name ?? trimmedName}」元信息`);
      await refreshBases();
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '更新知识库失败'));
    } finally {
      setUpdating(false);
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
            知识库与检索授权管理
          </Heading>
          <Text as="p" className="mt-1 text-sm text-muted-foreground">
            管理授权知识库元信息与可见范围，进入详情页分步登记资料、执行切片索引、发布或立即撤回。
          </Text>
        </Layout>

        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogTrigger asChild>
            <Button>
              <PlusIcon data-icon="inline-start" />
              新建知识库
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>新建知识库</DialogTitle>
              <DialogDescription>
                创建受控知识库容器。创建后可进入详情页分步登记文档或授权课程图文来源。
              </DialogDescription>
            </DialogHeader>
            <Layout
              as="form"
              onSubmit={(e) => void handleCreate(e)}
              className="space-y-5"
            >
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor="kb-create-name">知识库名称</FieldLabel>
                  <Input
                    id="kb-create-name"
                    placeholder="例如：Go 微服务架构核心资料库"
                    value={createName}
                    onChange={(e) => setCreateName(e.target.value)}
                    disabled={creating}
                    required
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="kb-create-visibility">
                    可见与检索范围
                  </FieldLabel>
                  <Select
                    value={createVisibility}
                    onValueChange={(val) =>
                      setCreateVisibility(val as AssistantApi.Visibility)
                    }
                    disabled={creating}
                  >
                    <SelectTrigger id="kb-create-visibility" className="w-full">
                      <SelectValue placeholder="选择可见范围" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectGroup>
                        <SelectItem value="public">
                          公开可见（所有已绑定 Agent 可按规则检索已发布来源）
                        </SelectItem>
                        <SelectItem value="private">
                          私有受限（按课程权限或归属范围校验后可检索）
                        </SelectItem>
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FieldDescription>
                    未授权或已撤回的来源均不会进入检索结果。
                  </FieldDescription>
                </Field>
                <Field>
                  <FieldLabel htmlFor="kb-create-desc">简介说明</FieldLabel>
                  <Textarea
                    id="kb-create-desc"
                    placeholder="说明该知识库收录的资料范围、适用课程或维护规范..."
                    value={createDescription}
                    onChange={(e) => setCreateDescription(e.target.value)}
                    disabled={creating}
                    rows={3}
                  />
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
                <Button type="submit" disabled={creating || !createName.trim()}>
                  {creating ? '创建中...' : '确认创建'}
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
          <AlertTitle>操作已完成</AlertTitle>
          <AlertDescription>{statusMsg}</AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-base">
            已登记知识库 ({bases.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          {bases.length === 0 ? (
            <Empty className="py-10">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <FolderOpenIcon />
                </EmptyMedia>
                <EmptyTitle>暂无知识库</EmptyTitle>
                <EmptyDescription>
                  当前尚未创建任何知识库，点击右上角「新建知识库」开始配置。
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                <Button onClick={() => setCreateOpen(true)}>
                  <PlusIcon data-icon="inline-start" />
                  新建知识库
                </Button>
              </EmptyContent>
            </Empty>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>知识库名称</TableHead>
                  <TableHead>可见范围</TableHead>
                  <TableHead>归属者</TableHead>
                  <TableHead>版本</TableHead>
                  <TableHead>创建时间</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {bases.map((kb) => (
                  <TableRow key={kb.id}>
                    <TableCell>
                      <Layout className="font-medium text-foreground">
                        {kb.name || '未命名知识库'}
                      </Layout>
                      {kb.description ? (
                        <Layout className="mt-0.5 line-clamp-1 max-w-md text-xs text-muted-foreground">
                          {kb.description}
                        </Layout>
                      ) : null}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          kb.visibility === 'public' ? 'default' : 'secondary'
                        }
                      >
                        {getKnowledgeVisibilityLabel(kb.visibility)}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {kb.ownerId || 'system'}
                    </TableCell>
                    <TableCell>
                      <Badge variant="ghost">v{kb.version ?? 1}</Badge>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {formatDateTime(kb.createdAt)}
                    </TableCell>
                    <TableCell className="text-right">
                      <Layout className="inline-flex flex-wrap items-center justify-end gap-2">
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => handleOpenEdit(kb)}
                        >
                          <PencilIcon data-icon="inline-start" />
                          编辑信息
                        </Button>
                        {kb.id ? (
                          <Button size="sm" asChild>
                            <Link href={`/manage/knowledge/${kb.id}`}>
                              管理来源与版本
                            </Link>
                          </Button>
                        ) : null}
                      </Layout>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Dialog
        open={Boolean(editTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setEditTarget(null);
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>编辑知识库信息</DialogTitle>
            <DialogDescription>
              更新知识库名称、简介与可见范围（当前版本 v
              {editTarget?.version ?? 1}）。
            </DialogDescription>
          </DialogHeader>
          <Layout
            as="form"
            onSubmit={(e) => void handleUpdate(e)}
            className="space-y-5"
          >
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="kb-edit-name">知识库名称</FieldLabel>
                <Input
                  id="kb-edit-name"
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  disabled={updating}
                  required
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="kb-edit-visibility">
                  可见与检索范围
                </FieldLabel>
                <Select
                  value={editVisibility}
                  onValueChange={(val) =>
                    setEditVisibility(val as AssistantApi.Visibility)
                  }
                  disabled={updating}
                >
                  <SelectTrigger id="kb-edit-visibility" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      <SelectItem value="public">公开可见</SelectItem>
                      <SelectItem value="private">私有受限</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel htmlFor="kb-edit-desc">简介说明</FieldLabel>
                <Textarea
                  id="kb-edit-desc"
                  value={editDescription}
                  onChange={(e) => setEditDescription(e.target.value)}
                  disabled={updating}
                  rows={3}
                />
              </Field>
            </FieldGroup>
            <DialogFooter>
              <Button
                type="button"
                variant="secondary"
                onClick={() => setEditTarget(null)}
                disabled={updating}
              >
                取消
              </Button>
              <Button type="submit" disabled={updating || !editName.trim()}>
                {updating ? '保存中...' : '保存修改'}
              </Button>
            </DialogFooter>
          </Layout>
        </DialogContent>
      </Dialog>
    </Layout>
  );
}
