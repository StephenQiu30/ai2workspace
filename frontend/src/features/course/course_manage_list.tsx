'use client';

import {Layout} from '@/components/layout/layout';
import {Text} from '@/components/layout/typography';
import {useState} from 'react';
import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {
  AlertCircleIcon,
  ArrowRightIcon,
  BookOpenIcon,
  CheckIcon,
  ExternalLinkIcon,
  PlusIcon,
  SendIcon,
  Trash2Icon,
  XIcon,
} from 'lucide-react';
import {toast} from 'sonner';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
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
import {Spinner} from '@/components/ui/spinner';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {Tabs, TabsList, TabsTrigger} from '@/components/ui/tabs';
import {Textarea} from '@/components/ui/textarea';
import {
  countCourseLessons,
  getCourseCombinedStatusMeta,
  getProductionStateMeta,
} from '@/features/course/course_status';
import {isAdmin, type SessionUser} from '@/lib/auth/roles';
import {
  COURSE_CATEGORIES,
  formatDateTime,
  getCourseCategoryLabel,
  getErrorMessage,
} from '@/lib/format';
import {
  learningCloseSales,
  learningCreateCourse,
  learningDeleteCourse,
  learningReviewCourse,
  learningSubmitCourse,
  learningWithdrawCourse,
} from '@/services/generated/learning/courses';

export interface CourseManageListProps {
  readonly initialCourses: LearningApi.CourseResponse[];
  readonly user: SessionUser;
  readonly loadFailed?: boolean;
}

type ManageTabValue =
  'all' | 'draft' | 'pending_review' | 'approved' | 'closed_or_withdrawn';

type GovernanceActionType =
  'submit' | 'approve' | 'reject' | 'close_sales' | 'withdraw' | 'delete';

interface PendingActionState {
  course: LearningApi.CourseResponse;
  action: GovernanceActionType;
}

const SELECTABLE_CATEGORIES = COURSE_CATEGORIES.filter(
  (item) => item.slug !== 'all',
);

export function CourseManageList({
  initialCourses,
  user,
  loadFailed = false,
}: CourseManageListProps) {
  const router = useRouter();
  const admin = isAdmin(user);

  const [tab, setTab] = useState<ManageTabValue>('all');
  const [createOpen, setCreateOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newTitle, setNewTitle] = useState('');
  const [newCategory, setNewCategory] = useState('ai');
  const [newDescription, setNewDescription] = useState('');
  const [newIsFree, setNewIsFree] = useState(false);

  const [pendingAction, setPendingAction] = useState<PendingActionState | null>(
    null,
  );
  const [actionReason, setActionReason] = useState('');
  const [actionSubmitting, setActionSubmitting] = useState(false);

  const filteredCourses = initialCourses.filter((course) => {
    switch (tab) {
      case 'draft':
        return (
          course.productionState === 'draft' ||
          course.productionState === 'rejected' ||
          !course.productionState
        );
      case 'pending_review':
        return course.productionState === 'pending_review';
      case 'approved':
        return (
          course.productionState === 'approved' &&
          course.saleState !== 'sales_closed' &&
          course.deliveryState !== 'withdrawn'
        );
      case 'closed_or_withdrawn':
        return (
          course.saleState === 'sales_closed' ||
          course.deliveryState === 'withdrawn'
        );
      case 'all':
      default:
        return true;
    }
  });

  async function handleCreateCourse(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const trimmedTitle = newTitle.trim();
    if (!trimmedTitle) {
      toast.error('请填写课程标题');
      return;
    }
    setCreating(true);
    try {
      const resp = await learningCreateCourse({
        title: trimmedTitle,
        category: newCategory,
        description: newDescription.trim(),
        isFree: newIsFree,
      });
      const createdId = resp.courseId;
      toast.success('课程草稿已创建');
      setCreateOpen(false);
      setNewTitle('');
      setNewDescription('');
      setNewIsFree(false);
      if (createdId) {
        router.push(`/manage/courses/${createdId}`);
      } else {
        router.refresh();
      }
    } catch (err: unknown) {
      toast.error(getErrorMessage(err, '创建课程草稿失败'));
    } finally {
      setCreating(false);
    }
  }

  function openActionDialog(
    course: LearningApi.CourseResponse,
    action: GovernanceActionType,
  ) {
    setActionReason('');
    setPendingAction({course, action});
  }

  async function handleConfirmGovernanceAction() {
    if (!pendingAction || !pendingAction.course.id) {
      return;
    }
    const {course, action} = pendingAction;
    const courseId = course.id ?? '';
    const expectedVersion = course.version ?? 1;

    setActionSubmitting(true);
    try {
      if (action === 'submit') {
        await learningSubmitCourse({id: courseId}, {expectedVersion});
        toast.success('已提交管理员审核');
      } else if (action === 'approve') {
        await learningReviewCourse(
          {id: courseId},
          {
            decision: 'approved',
            reason: actionReason.trim() || '审核通过并发布',
            expectedVersion,
          },
        );
        toast.success('课程已审核通过并发布');
      } else if (action === 'reject') {
        await learningReviewCourse(
          {id: courseId},
          {
            decision: 'rejected',
            reason: actionReason.trim() || '请补充章节与课时内容后再提交',
            expectedVersion,
          },
        );
        toast.success('已退回课程草稿');
      } else if (action === 'close_sales') {
        await learningCloseSales(
          {id: courseId},
          {
            reason: actionReason.trim() || '停止新售，保留已购学员访问',
            expectedVersion,
          },
        );
        toast.success('已停止新售，已购学员仍可继续学习');
      } else if (action === 'withdraw') {
        await learningWithdrawCourse(
          {id: courseId},
          {
            reason: actionReason.trim() || '因合规或质量原因撤回交付',
            expectedVersion,
          },
        );
        toast.success('课程已撤回交付');
      } else if (action === 'delete') {
        await learningDeleteCourse({
          id: courseId,
          expectedVersion,
        });
        toast.success('未发布草稿已删除');
      }
      setPendingAction(null);
      router.refresh();
    } catch (err: unknown) {
      toast.error(getErrorMessage(err, '执行课程操作失败，请刷新后重试'));
    } finally {
      setActionSubmitting(false);
    }
  }

  function getActionDialogCopy(action: GovernanceActionType): {
    title: string;
    description: string;
    requireReason: boolean;
    reasonPlaceholder: string;
    confirmLabel: string;
  } {
    switch (action) {
      case 'submit':
        return {
          title: '确认提交课程审核',
          description:
            '提交后课程将进入“待审核”状态，由平台管理员核对章节大纲、媒体转码与定价后发布上线。',
          requireReason: false,
          reasonPlaceholder: '',
          confirmLabel: '确认提交审核',
        };
      case 'approve':
        return {
          title: '审核通过并发布课程',
          description:
            '通过后该课程将立即更新发布版本并出现在公开课程目录中。若是付费课程，请确保已在课程编辑页设置人民币价格。',
          requireReason: false,
          reasonPlaceholder: '可选：填写审核通过备注…',
          confirmLabel: '通过并发布',
        };
      case 'reject':
        return {
          title: '退回课程修改',
          description:
            '退回后课程将回到“已退回”状态，讲师可在课程编辑页查看您的退回原因并修改后重新提交。',
          requireReason: true,
          reasonPlaceholder: '请说明需要讲师修改或补充的具体问题…',
          confirmLabel: '确认退回',
        };
      case 'close_sales':
        return {
          title: '停止课程新售',
          description:
            '停售后将关闭新购买与新报名入口，但此前已购买或已报名的学员仍可正常学习全部已发布内容。',
          requireReason: false,
          reasonPlaceholder: '可选：记录停售原因…',
          confirmLabel: '确认停售',
        };
      case 'withdraw':
        return {
          title: '撤回课程交付',
          description:
            '撤回后课程将立即从公开目录移除且停止一切课时访问，仅用于合规或严重交付故障处置。',
          requireReason: true,
          reasonPlaceholder: '请填写撤回交付的合规或故障原因…',
          confirmLabel: '确认撤回交付',
        };
      case 'delete':
        return {
          title: '删除未发布课程草稿',
          description: '删除后该课程草稿及其下属章节将被永久移除，不可恢复。',
          requireReason: false,
          reasonPlaceholder: '',
          confirmLabel: '确认删除草稿',
        };
    }
  }

  const dialogCopy = pendingAction
    ? getActionDialogCopy(pendingAction.action)
    : null;

  return (
    <Layout flow="stack" className="gap-6">
      <Layout
        flow="stack"
        className="justify-between gap-4 sm:flex-row sm:items-center"
      >
        <Tabs
          value={tab}
          onValueChange={(val) => setTab(val as ManageTabValue)}
        >
          <TabsList>
            <TabsTrigger value="all">
              全部 ({initialCourses.length})
            </TabsTrigger>
            <TabsTrigger value="draft">草稿 / 退回</TabsTrigger>
            <TabsTrigger value="pending_review">待审核</TabsTrigger>
            <TabsTrigger value="approved">已发布</TabsTrigger>
            <TabsTrigger value="closed_or_withdrawn">已停售 / 撤回</TabsTrigger>
          </TabsList>
        </Tabs>

        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogTrigger asChild>
            <Button type="button">
              <PlusIcon data-icon="inline-start" />
              新建课程
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>新建课程草稿</DialogTitle>
              <DialogDescription>
                填写课程基础信息并选择学科分类。创建后将自动进入独立课程编辑页编排章节与课时。
              </DialogDescription>
            </DialogHeader>

            <Layout
              as="form"
              flow="stack"
              onSubmit={(e) => void handleCreateCourse(e)}
              className="gap-4"
            >
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor="new-course-title">课程标题</FieldLabel>
                  <Input
                    id="new-course-title"
                    placeholder="例如：AI 短漫剧分镜"
                    value={newTitle}
                    onChange={(e) => setNewTitle(e.target.value)}
                    required
                  />
                </Field>

                <Field>
                  <FieldLabel htmlFor="new-course-category">
                    课程分类
                  </FieldLabel>
                  <Select value={newCategory} onValueChange={setNewCategory}>
                    <SelectTrigger id="new-course-category">
                      <SelectValue placeholder="选择分类" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectGroup>
                        {SELECTABLE_CATEGORIES.map((cat) => (
                          <SelectItem key={cat.slug} value={cat.slug}>
                            {cat.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </Field>

                <Field>
                  <FieldLabel htmlFor="new-course-desc">课程简介</FieldLabel>
                  <Textarea
                    id="new-course-desc"
                    placeholder="学员将完成什么作品？"
                    value={newDescription}
                    onChange={(e) => setNewDescription(e.target.value)}
                    rows={3}
                  />
                </Field>

                <Field orientation="horizontal">
                  <Checkbox
                    id="new-course-free"
                    checked={newIsFree}
                    onCheckedChange={(checked) =>
                      setNewIsFree(checked === true)
                    }
                  />
                  <Layout flow="stack" className="gap-0.5">
                    <FieldLabel htmlFor="new-course-free">免费课程</FieldLabel>
                    <FieldDescription>
                      付费课程由管理员设置售价。
                    </FieldDescription>
                  </Layout>
                </Field>

                <DialogFooter>
                  <Button
                    type="button"
                    variant="secondary"
                    disabled={creating}
                    onClick={() => setCreateOpen(false)}
                  >
                    取消
                  </Button>
                  <Button type="submit" disabled={creating}>
                    {creating ? <Spinner data-icon="inline-start" /> : null}
                    {creating ? '正在创建…' : '创建并进入编辑'}
                  </Button>
                </DialogFooter>
              </FieldGroup>
            </Layout>
          </DialogContent>
        </Dialog>
      </Layout>

      {loadFailed ? (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>课程管理列表加载失败</AlertTitle>
          <AlertDescription>
            无法从课程服务读取管理列表，请稍后刷新重试。
          </AlertDescription>
        </Alert>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>课程供给与审核台账</CardTitle>
          <CardDescription>
            {admin
              ? '您当前以平台管理员身份查看全站课程，可执行内容编辑、人民币定价、上线审核及停售/撤回处置。'
              : '管理您名下的课程草稿、章节课时与视频素材，完善后提交管理员审核发布。'}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {filteredCourses.length === 0 ? (
            <Empty className="bg-canvas-soft py-12">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <BookOpenIcon />
                </EmptyMedia>
                <EmptyTitle>当前筛选下暂无课程</EmptyTitle>
                <EmptyDescription>
                  点击右上角“新建课程”创建您的第一门课程草稿，或切换到其他状态标签查看。
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => setCreateOpen(true)}
                >
                  <PlusIcon data-icon="inline-start" />
                  新建课程草稿
                </Button>
              </EmptyContent>
            </Empty>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>课程名称</TableHead>
                  <TableHead>分类与售卖</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>课时 / 版本</TableHead>
                  <TableHead>更新时间</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredCourses.map((course, idx) => {
                  const courseId = course.id ?? `course-${idx}`;
                  const prodMeta = getProductionStateMeta(
                    course.productionState,
                  );
                  const combinedMeta = getCourseCombinedStatusMeta(course);
                  const lessonsCount = countCourseLessons(course);
                  const canSubmit =
                    course.productionState === 'draft' ||
                    course.productionState === 'rejected' ||
                    !course.productionState;
                  const canReview =
                    admin && course.productionState === 'pending_review';
                  const canCloseSales =
                    admin &&
                    course.productionState === 'approved' &&
                    course.saleState !== 'sales_closed' &&
                    course.deliveryState !== 'withdrawn';
                  const canWithdraw =
                    admin &&
                    course.productionState === 'approved' &&
                    course.deliveryState !== 'withdrawn';
                  const canDeleteDraft =
                    !course.publishedVersion &&
                    course.productionState !== 'approved';

                  return (
                    <TableRow key={courseId}>
                      <TableCell className="max-w-xs">
                        <Layout flow="stack" className="gap-1">
                          <Link
                            href={`/manage/courses/${courseId}`}
                            className="truncate font-medium text-foreground hover:underline"
                          >
                            {course.title || '未命名课程'}
                          </Link>
                          <Text className="truncate text-xs text-muted-foreground">
                            讲师：{course.authorNickname?.trim() || '学屿讲师'}
                          </Text>
                          {course.reviewReason ? (
                            <Text className="line-clamp-1 text-xs text-muted-foreground">
                              审核备注：{course.reviewReason}
                            </Text>
                          ) : null}
                        </Layout>
                      </TableCell>

                      <TableCell>
                        <Layout
                          flow="row"
                          className="flex-wrap items-center gap-1.5"
                        >
                          <Badge variant="secondary">
                            {getCourseCategoryLabel(course.category)}
                          </Badge>
                          <Badge
                            variant={course.isFree ? 'secondary' : 'default'}
                          >
                            {course.isFree ? '免费' : '买断'}
                          </Badge>
                        </Layout>
                      </TableCell>

                      <TableCell>
                        <Layout
                          flow="row"
                          className="flex-wrap items-center gap-1.5"
                        >
                          <Badge variant={combinedMeta.variant}>
                            {combinedMeta.label}
                          </Badge>
                          {combinedMeta.label !== prodMeta.label ? (
                            <Badge variant="secondary">{prodMeta.label}</Badge>
                          ) : null}
                        </Layout>
                      </TableCell>

                      <TableCell className="text-xs text-muted-foreground">
                        <Layout>{lessonsCount} 课时</Layout>
                        <Layout>
                          草稿 v{course.version ?? 1}
                          {course.publishedVersion
                            ? ` · 线上 v${course.publishedVersion}`
                            : ''}
                        </Layout>
                      </TableCell>

                      <TableCell className="text-xs text-muted-foreground">
                        {formatDateTime(course.updatedAt ?? course.createdAt)}
                      </TableCell>

                      <TableCell className="text-right">
                        <Layout
                          flow="row"
                          className="flex-wrap items-center justify-end gap-1.5"
                        >
                          <Button asChild variant="secondary" size="xs">
                            <Link href={`/manage/courses/${courseId}`}>
                              编辑内容
                              <ArrowRightIcon data-icon="inline-end" />
                            </Link>
                          </Button>

                          {course.productionState === 'approved' ? (
                            <Button asChild variant="ghost" size="xs">
                              <Link href={`/courses/${courseId}`}>
                                <ExternalLinkIcon data-icon="inline-start" />
                                公开页
                              </Link>
                            </Button>
                          ) : null}

                          {canSubmit ? (
                            <Button
                              type="button"
                              variant="secondary"
                              size="xs"
                              onClick={() => openActionDialog(course, 'submit')}
                            >
                              <SendIcon data-icon="inline-start" />
                              提审
                            </Button>
                          ) : null}

                          {canReview ? (
                            <>
                              <Button
                                type="button"
                                size="xs"
                                onClick={() =>
                                  openActionDialog(course, 'approve')
                                }
                              >
                                <CheckIcon data-icon="inline-start" />
                                通过
                              </Button>
                              <Button
                                type="button"
                                variant="secondary"
                                size="xs"
                                onClick={() =>
                                  openActionDialog(course, 'reject')
                                }
                              >
                                <XIcon data-icon="inline-start" />
                                退回
                              </Button>
                            </>
                          ) : null}

                          {canCloseSales ? (
                            <Button
                              type="button"
                              variant="ghost"
                              size="xs"
                              onClick={() =>
                                openActionDialog(course, 'close_sales')
                              }
                            >
                              停售
                            </Button>
                          ) : null}

                          {canWithdraw ? (
                            <Button
                              type="button"
                              variant="destructive"
                              size="xs"
                              onClick={() =>
                                openActionDialog(course, 'withdraw')
                              }
                            >
                              撤回
                            </Button>
                          ) : null}

                          {canDeleteDraft ? (
                            <Button
                              type="button"
                              variant="ghost"
                              size="xs"
                              onClick={() => openActionDialog(course, 'delete')}
                            >
                              <Trash2Icon data-icon="inline-start" />
                              删除
                            </Button>
                          ) : null}
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

      <AlertDialog
        open={Boolean(pendingAction)}
        onOpenChange={(open) => {
          if (!open && !actionSubmitting) {
            setPendingAction(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{dialogCopy?.title}</AlertDialogTitle>
            <AlertDialogDescription>
              {pendingAction?.course.title
                ? `目标课程：《${pendingAction.course.title}》 — `
                : ''}
              {dialogCopy?.description}
            </AlertDialogDescription>
          </AlertDialogHeader>

          {pendingAction &&
          pendingAction.action !== 'submit' &&
          pendingAction.action !== 'delete' ? (
            <Field>
              <FieldLabel htmlFor="governance-action-reason">
                {dialogCopy?.requireReason
                  ? '处理原因（必填）'
                  : '备注说明（可选）'}
              </FieldLabel>
              <Textarea
                id="governance-action-reason"
                rows={3}
                placeholder={dialogCopy?.reasonPlaceholder}
                value={actionReason}
                onChange={(e) => setActionReason(e.target.value)}
              />
            </Field>
          ) : null}

          <AlertDialogFooter>
            <AlertDialogCancel disabled={actionSubmitting}>
              取消
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={
                actionSubmitting ||
                Boolean(dialogCopy?.requireReason && !actionReason.trim())
              }
              onClick={(e) => {
                e.preventDefault();
                void handleConfirmGovernanceAction();
              }}
            >
              {actionSubmitting ? <Spinner data-icon="inline-start" /> : null}
              {dialogCopy?.confirmLabel}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Layout>
  );
}
