'use client';

import {useState} from 'react';
import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {
  AlertCircleIcon,
  ArrowLeftIcon,
  BookOpenIcon,
  CheckIcon,
  ExternalLinkIcon,
  FileTextIcon,
  PlayCircleIcon,
  PlusIcon,
  SaveIcon,
  SendIcon,
  Trash2Icon,
  XIcon,
} from 'lucide-react';
import {toast} from 'sonner';
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion';
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
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import {Field, FieldDescription, FieldLabel} from '@/components/ui/field';
import {Input} from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {Spinner} from '@/components/ui/spinner';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import {Textarea} from '@/components/ui/textarea';
import {TiptapEditor} from '@/components/editor/tiptap_editor';
import {
  getCourseCombinedStatusMeta,
  getDeliveryStateMeta,
  getProductionStateMeta,
  getSaleStateMeta,
  sortSectionsAndLessons,
} from '@/features/course/course_status';
import {VideoUploader} from '@/features/media/video_uploader';
import {isAdmin, type SessionUser} from '@/lib/auth/roles';
import {COURSE_CATEGORIES, formatPriceCny, getErrorMessage} from '@/lib/format';
import {commerceSetCoursePrice} from '@/services/generated/commerce/commerce';
import {
  learningAddLesson,
  learningAddSection,
  learningCloseSales,
  learningCreateCourse,
  learningDeleteLesson,
  learningDeleteSection,
  learningGetCourse,
  learningReviewCourse,
  learningSubmitCourse,
  learningUpdateCourse,
  learningUpdateLesson,
  learningUpdateSection,
  learningWithdrawCourse,
} from '@/services/generated/learning/courses';

export interface CourseEditorProps {
  readonly initialCourse?: LearningApi.CourseResponse | null;
  readonly initialPrice?: CommerceApi.PriceResponse | null;
  readonly user?: SessionUser | null;
}

type GovernanceDialogType =
  'submit' | 'approve' | 'reject' | 'close_sales' | 'withdraw';

const SELECTABLE_CATEGORIES = COURSE_CATEGORIES.filter(
  (item) => item.slug !== 'all',
);

export function CourseEditor({
  initialCourse = null,
  initialPrice = null,
  user = null,
}: CourseEditorProps) {
  const router = useRouter();
  const admin = isAdmin(user);

  const [course, setCourse] = useState<LearningApi.CourseResponse | null>(
    initialCourse,
  );
  const [courseId, setCourseId] = useState<string>(initialCourse?.id ?? '');
  const [title, setTitle] = useState<string>(initialCourse?.title ?? '');
  const [category, setCategory] = useState<string>(
    initialCourse?.category || 'ai',
  );
  const [description, setDescription] = useState<string>(
    initialCourse?.description ?? '',
  );
  const [isFree, setIsFree] = useState<boolean>(Boolean(initialCourse?.isFree));
  const [currentVersion, setCurrentVersion] = useState<number>(
    initialCourse?.version ?? 1,
  );
  const [productionState, setProductionState] = useState<string>(
    initialCourse?.productionState ?? 'draft',
  );
  const [saleState, setSaleState] = useState<string>(
    initialCourse?.saleState ?? 'on_sale',
  );
  const [deliveryState, setDeliveryState] = useState<string>(
    initialCourse?.deliveryState ?? 'available',
  );

  // Price state (CNY Yuan input -> minor units)
  const initialYuan =
    typeof initialPrice?.amountMinor === 'number' &&
    initialPrice.amountMinor > 0
      ? (initialPrice.amountMinor / 100).toString()
      : '';
  const [priceYuan, setPriceYuan] = useState<string>(initialYuan);
  const [priceMinor, setPriceMinor] = useState<number | undefined>(
    initialPrice?.amountMinor,
  );
  const [priceVersion, setPriceVersion] = useState<number | undefined>(
    initialPrice?.version,
  );
  const [savingPrice, setSavingPrice] = useState<boolean>(false);

  // Section creation & editing state
  const [newSectionTitle, setNewSectionTitle] = useState<string>('');
  const [editingSectionId, setEditingSectionId] = useState<string | null>(null);
  const [editingSectionTitle, setEditingSectionTitle] = useState<string>('');
  const [deletingSectionId, setDeletingSectionId] = useState<string | null>(
    null,
  );

  // Lesson creation & editing state
  const sortedSections = sortSectionsAndLessons(course?.sections);
  const [targetSectionId, setTargetSectionId] = useState<string>(
    sortedSections[0]?.id ?? '',
  );
  const [selectedLessonId, setSelectedLessonId] = useState<string | null>(null);
  const [lessonTitle, setLessonTitle] = useState<string>('');
  const [lessonType, setLessonType] = useState<'video' | 'article'>('video');
  const [isTrial, setIsTrial] = useState<boolean>(false);
  const [articleContent, setArticleContent] = useState<string>('');
  const [currentLessonMediaVersionId, setCurrentLessonMediaVersionId] =
    useState<string | undefined>(undefined);
  const [deletingLessonId, setDeletingLessonId] = useState<string | null>(null);

  // Governance dialog state
  const [governanceDialog, setGovernanceDialog] =
    useState<GovernanceDialogType | null>(null);
  const [governanceReason, setGovernanceReason] = useState<string>('');

  // Status feedback
  const [loading, setLoading] = useState<boolean>(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  async function reloadCourseDetails(targetId = courseId) {
    if (!targetId) {
      return;
    }
    try {
      const fresh = await learningGetCourse({id: targetId});
      setCourse(fresh);
      if (typeof fresh.version === 'number') {
        setCurrentVersion(fresh.version);
      }
      if (fresh.productionState) {
        setProductionState(fresh.productionState);
      }
      if (fresh.saleState) {
        setSaleState(fresh.saleState);
      }
      if (fresh.deliveryState) {
        setDeliveryState(fresh.deliveryState);
      }
      const nextSections = sortSectionsAndLessons(fresh.sections);
      if (
        nextSections.length > 0 &&
        (!targetSectionId ||
          !nextSections.some((item) => item.id === targetSectionId))
      ) {
        setTargetSectionId(nextSections[0]?.id ?? '');
      }
    } catch {
      router.refresh();
    }
  }

  async function handleSaveBasicInfo(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setErrorMessage(null);
    const trimmedTitle = title.trim();
    if (!trimmedTitle) {
      setErrorMessage('请填写课程标题');
      return;
    }

    setLoading(true);
    try {
      if (!courseId) {
        const created = await learningCreateCourse({
          title: trimmedTitle,
          category,
          description: description.trim(),
          isFree,
        });
        const newId = created.courseId ?? '';
        setCourseId(newId);
        setCurrentVersion(created.version ?? 1);
        setProductionState(created.state ?? 'draft');
        toast.success('课程草稿已创建');
        if (newId) {
          router.push(`/manage/courses/${newId}`);
        }
      } else {
        const updated = await learningUpdateCourse(
          {id: courseId},
          {
            title: trimmedTitle,
            category,
            description: description.trim(),
            isFree,
            expectedVersion: currentVersion,
          },
        );
        setCurrentVersion(updated.version ?? currentVersion + 1);
        if (updated.state) {
          setProductionState(updated.state);
        }
        toast.success('课程基本信息已保存');
        await reloadCourseDetails(courseId);
      }
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '保存课程基本信息失败');
      setErrorMessage(msg);
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  }

  async function handleSaveCoursePrice(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!courseId) {
      return;
    }
    const parsedYuan = Number.parseFloat(priceYuan);
    if (!Number.isFinite(parsedYuan) || parsedYuan <= 0) {
      toast.error('请输入大于 0 的人民币金额（元）');
      return;
    }
    const amountMinor = Math.round(parsedYuan * 100);
    setSavingPrice(true);
    setErrorMessage(null);
    try {
      const resp = await commerceSetCoursePrice(
        {id: courseId},
        {
          amountMinor,
          currency: 'CNY',
          expectedVersion: priceVersion,
        },
      );
      setPriceMinor(resp.amountMinor ?? amountMinor);
      setPriceVersion(resp.version);
      toast.success(`课程定价已更新为 ${formatPriceCny(amountMinor, false)}`);
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '保存课程定价失败');
      setErrorMessage(msg);
      toast.error(msg);
    } finally {
      setSavingPrice(false);
    }
  }

  async function handleAddSection(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!courseId || !newSectionTitle.trim()) {
      return;
    }
    setLoading(true);
    setErrorMessage(null);
    try {
      const nextSort = sortedSections.length + 1;
      const resp = await learningAddSection(
        {id: courseId},
        {
          title: newSectionTitle.trim(),
          sortOrder: nextSort,
        },
      );
      setNewSectionTitle('');
      if (resp.id) {
        setTargetSectionId(resp.id);
      }
      toast.success(`已添加章节「${resp.title ?? '新章节'}」`);
      await reloadCourseDetails(courseId);
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '添加章节失败');
      setErrorMessage(msg);
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  }

  async function handleUpdateSectionTitle(
    sectionId: string,
    sortOrder?: number,
  ) {
    if (!courseId || !editingSectionTitle.trim()) {
      return;
    }
    setLoading(true);
    setErrorMessage(null);
    try {
      await learningUpdateSection(
        {id: courseId, sectionId},
        {
          title: editingSectionTitle.trim(),
          sortOrder,
        },
      );
      setEditingSectionId(null);
      setEditingSectionTitle('');
      toast.success('章节名称已更新');
      await reloadCourseDetails(courseId);
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '修改章节失败');
      setErrorMessage(msg);
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  }

  async function handleConfirmDeleteSection() {
    if (!courseId || !deletingSectionId) {
      return;
    }
    setLoading(true);
    setErrorMessage(null);
    try {
      await learningDeleteSection({
        id: courseId,
        sectionId: deletingSectionId,
      });
      setDeletingSectionId(null);
      toast.success('章节已删除');
      await reloadCourseDetails(courseId);
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '删除章节失败');
      setErrorMessage(msg);
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  }

  function startCreateLesson(sectionId?: string) {
    setSelectedLessonId(null);
    if (sectionId) {
      setTargetSectionId(sectionId);
    } else if (!targetSectionId && sortedSections[0]?.id) {
      setTargetSectionId(sortedSections[0].id);
    }
    setLessonTitle('');
    setLessonType('video');
    setIsTrial(false);
    setArticleContent('');
    setCurrentLessonMediaVersionId(undefined);
  }

  function startEditLesson(lesson: LearningApi.Lesson, sectionId?: string) {
    setSelectedLessonId(lesson.id ?? null);
    setTargetSectionId(lesson.sectionId || sectionId || '');
    setLessonTitle(lesson.title ?? '');
    setLessonType(lesson.type === 'article' ? 'article' : 'video');
    setIsTrial(Boolean(lesson.isTrial));
    setArticleContent(lesson.content ?? '');
    setCurrentLessonMediaVersionId(lesson.mediaVersionId);
  }

  async function handleSaveLesson(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!courseId) {
      return;
    }
    const secId = targetSectionId || sortedSections[0]?.id || '';
    if (!secId) {
      toast.error('请先创建一个章节后再添加课时');
      return;
    }
    if (!lessonTitle.trim()) {
      toast.error('请填写课时名称');
      return;
    }

    setLoading(true);
    setErrorMessage(null);
    try {
      if (selectedLessonId) {
        const updated = await learningUpdateLesson(
          {id: courseId, lessonId: selectedLessonId},
          {
            sectionId: secId,
            title: lessonTitle.trim(),
            type: lessonType,
            isTrial,
            content: lessonType === 'article' ? articleContent : undefined,
          },
        );
        setCurrentLessonMediaVersionId(updated.mediaVersionId);
        toast.success(`课时「${updated.title ?? lessonTitle}」已更新`);
      } else {
        const targetSec = sortedSections.find((item) => item.id === secId);
        const nextSort = (targetSec?.lessons?.length ?? 0) + 1;
        const created = await learningAddLesson(
          {id: courseId},
          {
            sectionId: secId,
            title: lessonTitle.trim(),
            type: lessonType,
            isTrial,
            content: lessonType === 'article' ? articleContent : '',
            sortOrder: nextSort,
          },
        );
        if (created.id) {
          setSelectedLessonId(created.id);
        }
        toast.success(`课时「${created.title ?? lessonTitle}」已创建`);
      }
      await reloadCourseDetails(courseId);
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '保存课时失败');
      setErrorMessage(msg);
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  }

  async function handleConfirmDeleteLesson() {
    if (!courseId || !deletingLessonId) {
      return;
    }
    setLoading(true);
    setErrorMessage(null);
    try {
      await learningDeleteLesson({
        id: courseId,
        lessonId: deletingLessonId,
      });
      if (selectedLessonId === deletingLessonId) {
        startCreateLesson();
      }
      setDeletingLessonId(null);
      toast.success('课时已删除');
      await reloadCourseDetails(courseId);
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '删除课时失败');
      setErrorMessage(msg);
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  }

  async function handleConfirmGovernance() {
    if (!courseId || !governanceDialog) {
      return;
    }
    setLoading(true);
    setErrorMessage(null);
    try {
      if (governanceDialog === 'submit') {
        const resp = await learningSubmitCourse(
          {id: courseId},
          {expectedVersion: currentVersion},
        );
        setCurrentVersion(resp.version ?? currentVersion + 1);
        setProductionState(resp.state ?? 'pending_review');
        toast.success('课程已提交管理员审核');
      } else if (governanceDialog === 'approve') {
        const resp = await learningReviewCourse(
          {id: courseId},
          {
            decision: 'approved',
            reason: governanceReason.trim() || '审核通过并发布上线',
            expectedVersion: currentVersion,
          },
        );
        setCurrentVersion(resp.version ?? currentVersion + 1);
        setProductionState(resp.state ?? 'approved');
        toast.success('课程已审核通过并发布');
      } else if (governanceDialog === 'reject') {
        const resp = await learningReviewCourse(
          {id: courseId},
          {
            decision: 'rejected',
            reason: governanceReason.trim() || '请完善课程章节与课时内容',
            expectedVersion: currentVersion,
          },
        );
        setCurrentVersion(resp.version ?? currentVersion + 1);
        setProductionState(resp.state ?? 'rejected');
        toast.success('已退回课程草稿');
      } else if (governanceDialog === 'close_sales') {
        const resp = await learningCloseSales(
          {id: courseId},
          {
            reason: governanceReason.trim() || '停止新售，保留已购学员访问',
            expectedVersion: currentVersion,
          },
        );
        setCurrentVersion(resp.version ?? currentVersion + 1);
        setSaleState('sales_closed');
        toast.success('课程已停止新售');
      } else if (governanceDialog === 'withdraw') {
        const resp = await learningWithdrawCourse(
          {id: courseId},
          {
            reason: governanceReason.trim() || '合规撤回交付',
            expectedVersion: currentVersion,
          },
        );
        setCurrentVersion(resp.version ?? currentVersion + 1);
        setDeliveryState('withdrawn');
        toast.success('课程已撤回交付');
      }
      setGovernanceDialog(null);
      setGovernanceReason('');
      await reloadCourseDetails(courseId);
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '执行治理动作失败');
      setErrorMessage(msg);
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  }

  const prodMeta = getProductionStateMeta(productionState);
  const saleMeta = getSaleStateMeta(saleState);
  const deliveryMeta = getDeliveryStateMeta(deliveryState);
  const combinedMeta = getCourseCombinedStatusMeta({
    productionState,
    saleState,
    deliveryState,
  });

  const defaultOpenSections = sortedSections.map(
    (sec, idx) => sec.id ?? `sec-${idx}`,
  );

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col justify-between gap-4 rounded-xl bg-canvas-soft p-5 sm:flex-row sm:items-center">
        <div className="flex flex-col gap-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={combinedMeta.variant}>{combinedMeta.label}</Badge>
            <Badge variant="secondary">草稿 v{currentVersion}</Badge>
            {course?.publishedVersion ? (
              <Badge variant="secondary">
                线上发布 v{course.publishedVersion}
              </Badge>
            ) : null}
            <Badge variant={isFree ? 'secondary' : 'default'}>
              {isFree
                ? '免费公开课'
                : `买断定价：${formatPriceCny(priceMinor, false)}`}
            </Badge>
          </div>
          <h2 className="text-[20px] leading-[30px] font-medium text-foreground">
            {title || '新建课程草稿'}
          </h2>
          {course?.reviewReason ? (
            <p className="text-xs text-muted-foreground">
              最近审核意见：{course.reviewReason}
            </p>
          ) : (
            <p className="text-xs text-muted-foreground">
              {combinedMeta.description}
            </p>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button asChild variant="secondary" size="sm">
            <Link href="/manage/courses">
              <ArrowLeftIcon data-icon="inline-start" />
              返回课程列表
            </Link>
          </Button>
          {courseId ? (
            <Button asChild variant="ghost" size="sm">
              <Link href={`/courses/${courseId}`}>
                <ExternalLinkIcon data-icon="inline-start" />
                预览课程详情页
              </Link>
            </Button>
          ) : null}
        </div>
      </div>

      {errorMessage ? (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>操作提示</AlertTitle>
          <AlertDescription>{errorMessage}</AlertDescription>
        </Alert>
      ) : null}

      <Tabs defaultValue="basic" className="flex flex-col gap-6">
        <TabsList>
          <TabsTrigger value="basic">基本信息与定价</TabsTrigger>
          <TabsTrigger value="curriculum" disabled={!courseId}>
            章节与课时 ({sortedSections.length})
          </TabsTrigger>
          <TabsTrigger value="governance" disabled={!courseId}>
            提交与审核发布
          </TabsTrigger>
        </TabsList>

        <TabsContent value="basic">
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
            <div className="lg:col-span-7">
              <Card>
                <CardHeader>
                  <CardTitle>课程基础信息</CardTitle>
                  <CardDescription>
                    设置课程标题、学科分类、简介及免费/付费售卖形态。
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <form
                    onSubmit={(e) => void handleSaveBasicInfo(e)}
                    className="flex flex-col gap-5"
                  >
                    <Field>
                      <FieldLabel htmlFor="course-editor-title">
                        课程标题
                      </FieldLabel>
                      <Input
                        id="course-editor-title"
                        placeholder="例如：Next.js 与 Go 微服务工程实战"
                        value={title}
                        onChange={(e) => setTitle(e.target.value)}
                        required
                      />
                    </Field>

                    <Field>
                      <FieldLabel htmlFor="course-editor-category">
                        课程分类
                      </FieldLabel>
                      <Select value={category} onValueChange={setCategory}>
                        <SelectTrigger id="course-editor-category">
                          <SelectValue placeholder="选择学科分类" />
                        </SelectTrigger>
                        <SelectContent>
                          {SELECTABLE_CATEGORIES.map((item) => (
                            <SelectItem key={item.slug} value={item.slug}>
                              {item.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <FieldDescription>
                        公开课程目录支持按此分类筛选。
                      </FieldDescription>
                    </Field>

                    <Field>
                      <FieldLabel htmlFor="course-editor-desc">
                        课程简介
                      </FieldLabel>
                      <Textarea
                        id="course-editor-desc"
                        rows={4}
                        placeholder="介绍课程的适用对象、核心模块与实战交付产出…"
                        value={description}
                        onChange={(e) => setDescription(e.target.value)}
                      />
                    </Field>

                    <Field orientation="horizontal">
                      <Checkbox
                        id="course-editor-free"
                        checked={isFree}
                        onCheckedChange={(checked) =>
                          setIsFree(checked === true)
                        }
                      />
                      <div className="flex flex-col gap-0.5">
                        <FieldLabel htmlFor="course-editor-free">
                          标记为免费公开课程
                        </FieldLabel>
                        <FieldDescription>
                          开启后学员登录即可免费报名学习；关闭则需按管理员核定的人民币买断价格购买。
                        </FieldDescription>
                      </div>
                    </Field>

                    <div className="flex items-center gap-3 pt-2">
                      <Button type="submit" disabled={loading}>
                        {loading ? (
                          <Spinner data-icon="inline-start" />
                        ) : (
                          <SaveIcon data-icon="inline-start" />
                        )}
                        {courseId ? '保存基本信息' : '创建课程草稿'}
                      </Button>
                    </div>
                  </form>
                </CardContent>
              </Card>
            </div>

            <div className="lg:col-span-5">
              <Card>
                <CardHeader>
                  <CardTitle>人民币（CNY）单课买断定价</CardTitle>
                  <CardDescription>
                    付费课程由平台管理员统一核定人民币售价，订单结算金额以服务端定价为准。
                  </CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-4">
                  <div className="rounded-xl bg-canvas-soft p-4">
                    <div className="text-xs text-muted-foreground">
                      当前生效价格
                    </div>
                    <div className="mt-1 text-2xl font-medium text-foreground">
                      {formatPriceCny(priceMinor, isFree)}
                    </div>
                  </div>

                  {isFree ? (
                    <p className="text-xs leading-5 text-muted-foreground">
                      当前课程已设为“免费公开课程”，无需设置买断金额。若要改为付费买断课，请先在左侧取消勾选免费开关并保存。
                    </p>
                  ) : admin && courseId ? (
                    <form
                      onSubmit={(e) => void handleSaveCoursePrice(e)}
                      className="flex flex-col gap-4"
                    >
                      <Field>
                        <FieldLabel htmlFor="course-price-yuan">
                          买断价格（人民币元）
                        </FieldLabel>
                        <Input
                          id="course-price-yuan"
                          type="number"
                          step="0.01"
                          min="0.01"
                          placeholder="例如：199"
                          value={priceYuan}
                          onChange={(e) => setPriceYuan(e.target.value)}
                          required
                        />
                        <FieldDescription>
                          支持精确到分（例如 99 或 199.00），单位为 CNY。
                        </FieldDescription>
                      </Field>
                      <Button
                        type="submit"
                        variant="secondary"
                        disabled={savingPrice}
                      >
                        {savingPrice ? (
                          <Spinner data-icon="inline-start" />
                        ) : null}
                        {savingPrice ? '正在保存定价…' : '更新课程售价'}
                      </Button>
                    </form>
                  ) : (
                    <p className="text-xs leading-5 text-muted-foreground">
                      讲师负责课程内容与试看配置；提交审核后，平台管理员将在此处核定并配置人民币买断售价。
                    </p>
                  )}
                </CardContent>
              </Card>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="curriculum">
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
            <div className="flex flex-col gap-4 lg:col-span-5">
              <Card>
                <CardHeader>
                  <CardTitle>章节与课时树</CardTitle>
                  <CardDescription>
                    先新增章节，再在章节下创建或点击编辑视频/图文课时。
                  </CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-5">
                  <form
                    onSubmit={(e) => void handleAddSection(e)}
                    className="flex items-end gap-2"
                  >
                    <Field className="flex-1">
                      <FieldLabel htmlFor="add-section-title">
                        新增章节名称
                      </FieldLabel>
                      <Input
                        id="add-section-title"
                        placeholder="例如：第一章 核心架构与环境准备"
                        value={newSectionTitle}
                        onChange={(e) => setNewSectionTitle(e.target.value)}
                        required
                      />
                    </Field>
                    <Button
                      type="submit"
                      variant="secondary"
                      disabled={loading}
                    >
                      <PlusIcon data-icon="inline-start" />
                      添加章节
                    </Button>
                  </form>

                  {sortedSections.length === 0 ? (
                    <Empty className="bg-canvas-soft py-10">
                      <EmptyHeader>
                        <EmptyMedia variant="icon">
                          <BookOpenIcon />
                        </EmptyMedia>
                        <EmptyTitle>暂未创建章节</EmptyTitle>
                        <EmptyDescription>
                          请先在上方输入章节标题并点击“添加章节”。
                        </EmptyDescription>
                      </EmptyHeader>
                    </Empty>
                  ) : (
                    <Accordion
                      type="multiple"
                      defaultValue={defaultOpenSections}
                      className="flex flex-col gap-2.5"
                    >
                      {sortedSections.map((sec, sIdx) => {
                        const secId = sec.id ?? `sec-${sIdx}`;
                        const lessons = sec.lessons ?? [];
                        const isEditingThisSection =
                          editingSectionId === sec.id;

                        return (
                          <AccordionItem
                            key={secId}
                            value={secId}
                            className="rounded-xl bg-canvas-soft px-3.5 py-1 not-last:border-b-0"
                          >
                            <AccordionTrigger className="py-3 text-sm font-medium text-foreground hover:no-underline">
                              <span className="truncate pr-2 text-left">
                                第 {sIdx + 1} 章 · {sec.title}（{lessons.length}
                                ）
                              </span>
                            </AccordionTrigger>
                            <AccordionContent className="flex flex-col gap-3 pb-3">
                              {isEditingThisSection && sec.id ? (
                                <div className="flex items-center gap-2">
                                  <Input
                                    value={editingSectionTitle}
                                    onChange={(e) =>
                                      setEditingSectionTitle(e.target.value)
                                    }
                                    placeholder="章节名称"
                                  />
                                  <Button
                                    type="button"
                                    size="xs"
                                    onClick={() =>
                                      void handleUpdateSectionTitle(
                                        sec.id ?? '',
                                        sec.sortOrder,
                                      )
                                    }
                                  >
                                    保存
                                  </Button>
                                  <Button
                                    type="button"
                                    variant="ghost"
                                    size="xs"
                                    onClick={() => setEditingSectionId(null)}
                                  >
                                    取消
                                  </Button>
                                </div>
                              ) : (
                                <div className="flex flex-wrap items-center justify-between gap-2">
                                  <Button
                                    type="button"
                                    variant="secondary"
                                    size="xs"
                                    onClick={() => startCreateLesson(sec.id)}
                                  >
                                    <PlusIcon data-icon="inline-start" />
                                    在本章新增课时
                                  </Button>
                                  <div className="flex items-center gap-1">
                                    <Button
                                      type="button"
                                      variant="ghost"
                                      size="xs"
                                      onClick={() => {
                                        setEditingSectionId(sec.id ?? null);
                                        setEditingSectionTitle(sec.title ?? '');
                                      }}
                                    >
                                      重命名
                                    </Button>
                                    {sec.id ? (
                                      <Button
                                        type="button"
                                        variant="ghost"
                                        size="xs"
                                        onClick={() =>
                                          setDeletingSectionId(sec.id ?? null)
                                        }
                                      >
                                        <Trash2Icon data-icon="inline-start" />
                                        删除章
                                      </Button>
                                    ) : null}
                                  </div>
                                </div>
                              )}

                              {lessons.length === 0 ? (
                                <p className="text-xs text-muted-foreground">
                                  本章暂无课时。
                                </p>
                              ) : (
                                <ul className="flex flex-col gap-1">
                                  {lessons.map((lsn, lIdx) => {
                                    const isSelected =
                                      selectedLessonId === lsn.id;
                                    const isVideo = lsn.type !== 'article';
                                    return (
                                      <li key={lsn.id ?? `${secId}-${lIdx}`}>
                                        <div
                                          className={`flex items-center justify-between gap-2 rounded-lg px-3 py-2 text-xs transition-colors ${
                                            isSelected
                                              ? 'bg-primary text-primary-foreground'
                                              : 'bg-background text-foreground hover:bg-secondary'
                                          }`}
                                        >
                                          <button
                                            type="button"
                                            onClick={() =>
                                              startEditLesson(lsn, sec.id)
                                            }
                                            className="flex min-w-0 flex-1 items-center gap-2 text-left"
                                          >
                                            {isVideo ? (
                                              <PlayCircleIcon className="size-3.5 shrink-0" />
                                            ) : (
                                              <FileTextIcon className="size-3.5 shrink-0" />
                                            )}
                                            <span className="truncate font-medium">
                                              {sIdx + 1}.{lIdx + 1} {lsn.title}
                                            </span>
                                          </button>
                                          <div className="flex shrink-0 items-center gap-1.5">
                                            {lsn.isTrial ? (
                                              <Badge variant="secondary">
                                                试看
                                              </Badge>
                                            ) : null}
                                            {lsn.id ? (
                                              <Button
                                                type="button"
                                                variant="ghost"
                                                size="icon-xs"
                                                aria-label={`删除课时 ${lsn.title ?? ''}`}
                                                onClick={() =>
                                                  setDeletingLessonId(
                                                    lsn.id ?? null,
                                                  )
                                                }
                                              >
                                                <Trash2Icon />
                                              </Button>
                                            ) : null}
                                          </div>
                                        </div>
                                      </li>
                                    );
                                  })}
                                </ul>
                              )}
                            </AccordionContent>
                          </AccordionItem>
                        );
                      })}
                    </Accordion>
                  )}
                </CardContent>
              </Card>
            </div>

            <div className="flex flex-col gap-6 lg:col-span-7">
              <Card>
                <CardHeader>
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div>
                      <CardTitle>
                        {selectedLessonId ? '编辑课时内容' : '新建课时'}
                      </CardTitle>
                      <CardDescription>
                        支持视频录播（H.264/HLS）与 Tiptap
                        富文本图文讲义两种课时形态。
                      </CardDescription>
                    </div>
                    {selectedLessonId ? (
                      <Button
                        type="button"
                        variant="secondary"
                        size="xs"
                        onClick={() => startCreateLesson()}
                      >
                        <PlusIcon data-icon="inline-start" />
                        切换到新建课时
                      </Button>
                    ) : null}
                  </div>
                </CardHeader>
                <CardContent className="flex flex-col gap-5">
                  <form
                    onSubmit={(e) => void handleSaveLesson(e)}
                    className="flex flex-col gap-5"
                  >
                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                      <Field>
                        <FieldLabel htmlFor="lesson-section-select">
                          所属章节
                        </FieldLabel>
                        <Select
                          value={targetSectionId}
                          onValueChange={setTargetSectionId}
                          disabled={sortedSections.length === 0}
                        >
                          <SelectTrigger id="lesson-section-select">
                            <SelectValue placeholder="请先在左侧添加章节" />
                          </SelectTrigger>
                          <SelectContent>
                            {sortedSections.map((sec, idx) =>
                              sec.id ? (
                                <SelectItem key={sec.id} value={sec.id}>
                                  第 {idx + 1} 章 · {sec.title}
                                </SelectItem>
                              ) : null,
                            )}
                          </SelectContent>
                        </Select>
                      </Field>

                      <Field>
                        <FieldLabel htmlFor="lesson-type-select">
                          课时类型
                        </FieldLabel>
                        <Select
                          value={lessonType}
                          onValueChange={(val) =>
                            setLessonType(val as 'video' | 'article')
                          }
                        >
                          <SelectTrigger id="lesson-type-select">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="video">
                              视频课时 (video)
                            </SelectItem>
                            <SelectItem value="article">
                              图文课时 (article)
                            </SelectItem>
                          </SelectContent>
                        </Select>
                      </Field>
                    </div>

                    <Field>
                      <FieldLabel htmlFor="lesson-title-input">
                        课时名称
                      </FieldLabel>
                      <Input
                        id="lesson-title-input"
                        placeholder="例如：1.1 领域边界划分与接口契约"
                        value={lessonTitle}
                        onChange={(e) => setLessonTitle(e.target.value)}
                        required
                      />
                    </Field>

                    <Field orientation="horizontal">
                      <Checkbox
                        id="lesson-trial-checkbox"
                        checked={isTrial}
                        onCheckedChange={(checked) =>
                          setIsTrial(checked === true)
                        }
                      />
                      <div className="flex flex-col gap-0.5">
                        <FieldLabel htmlFor="lesson-trial-checkbox">
                          设为免费试看 / 试读课时
                        </FieldLabel>
                        <FieldDescription>
                          开启后未购课学员与游客可直接打开本课时体验。
                        </FieldDescription>
                      </div>
                    </Field>

                    {lessonType === 'article' ? (
                      <Field>
                        <FieldLabel>图文课时富文本正文</FieldLabel>
                        <TiptapEditor
                          mode="article"
                          value={articleContent}
                          onChange={setArticleContent}
                          placeholder="编写图文课时讲义，支持二级/三级标题、代码块、列表与配图…"
                          ariaLabel="图文课时富文本正文"
                        />
                      </Field>
                    ) : null}

                    <div className="flex flex-wrap items-center gap-3">
                      <Button
                        type="submit"
                        disabled={loading || sortedSections.length === 0}
                      >
                        {loading ? (
                          <Spinner data-icon="inline-start" />
                        ) : (
                          <SaveIcon data-icon="inline-start" />
                        )}
                        {selectedLessonId ? '保存课时修改' : '创建课时'}
                      </Button>
                    </div>
                  </form>

                  {lessonType === 'video' ? (
                    selectedLessonId ? (
                      <VideoUploader
                        courseId={courseId}
                        lessonId={selectedLessonId}
                        currentMediaVersionId={currentLessonMediaVersionId}
                        onPublished={(mvId) => {
                          setCurrentLessonMediaVersionId(mvId);
                          void reloadCourseDetails(courseId);
                        }}
                      />
                    ) : (
                      <div className="rounded-xl bg-canvas-soft p-4 text-xs text-muted-foreground">
                        请先填写课时名称并点击“创建课时”，生成课时记录后即可上传并绑定
                        MP4 视频素材。
                      </div>
                    )
                  ) : null}
                </CardContent>
              </Card>
            </div>
          </div>
        </TabsContent>

        <TabsContent value="governance">
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
            <div className="lg:col-span-6">
              <Card>
                <CardHeader>
                  <CardTitle>课程状态与讲师提审</CardTitle>
                  <CardDescription>
                    核对制作状态、售卖状态与交付状态，讲师完善大纲后可在此提交管理员审核。
                  </CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-4">
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                    <div className="rounded-xl bg-canvas-soft p-3.5">
                      <div className="text-xs text-muted-foreground">
                        制作审核状态
                      </div>
                      <div className="mt-1.5">
                        <Badge variant={prodMeta.variant}>
                          {prodMeta.label}
                        </Badge>
                      </div>
                    </div>
                    <div className="rounded-xl bg-canvas-soft p-3.5">
                      <div className="text-xs text-muted-foreground">
                        售卖状态
                      </div>
                      <div className="mt-1.5">
                        <Badge variant={saleMeta.variant}>
                          {saleMeta.label}
                        </Badge>
                      </div>
                    </div>
                    <div className="rounded-xl bg-canvas-soft p-3.5">
                      <div className="text-xs text-muted-foreground">
                        内容交付状态
                      </div>
                      <div className="mt-1.5">
                        <Badge variant={deliveryMeta.variant}>
                          {deliveryMeta.label}
                        </Badge>
                      </div>
                    </div>
                  </div>

                  {course?.reviewReason ? (
                    <div className="rounded-xl bg-secondary p-4 text-xs leading-5 text-body">
                      <div className="font-medium text-foreground">
                        管理员审核意见
                      </div>
                      <p className="mt-1">{course.reviewReason}</p>
                    </div>
                  ) : null}

                  <div className="flex flex-wrap items-center gap-3 pt-2">
                    <Button
                      type="button"
                      disabled={loading || productionState === 'pending_review'}
                      onClick={() => {
                        setGovernanceReason('');
                        setGovernanceDialog('submit');
                      }}
                    >
                      <SendIcon data-icon="inline-start" />
                      {productionState === 'pending_review'
                        ? '已提交审核，等待管理员处理'
                        : '提交管理员审核'}
                    </Button>
                  </div>
                </CardContent>
              </Card>
            </div>

            <div className="lg:col-span-6">
              <Card>
                <CardHeader>
                  <CardTitle>平台管理员审核与上下架治理</CardTitle>
                  <CardDescription>
                    管理员专用：执行审核通过发布、退回草稿、停止新售（保留已购访问）或合规撤回交付。
                  </CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-4">
                  {!admin ? (
                    <p className="text-xs leading-5 text-muted-foreground">
                      您当前以讲师身份登录。提交课程审核后，平台管理员将在此处完成审核发布或上下架处置。
                    </p>
                  ) : (
                    <>
                      <div className="flex flex-col gap-2">
                        <div className="text-xs font-medium text-foreground">
                          1. 内容审核决议
                        </div>
                        <div className="flex flex-wrap items-center gap-2">
                          <Button
                            type="button"
                            size="sm"
                            disabled={loading}
                            onClick={() => {
                              setGovernanceReason('');
                              setGovernanceDialog('approve');
                            }}
                          >
                            <CheckIcon data-icon="inline-start" />
                            审核通过并发布
                          </Button>
                          <Button
                            type="button"
                            variant="secondary"
                            size="sm"
                            disabled={loading}
                            onClick={() => {
                              setGovernanceReason('');
                              setGovernanceDialog('reject');
                            }}
                          >
                            <XIcon data-icon="inline-start" />
                            退回讲师修改
                          </Button>
                        </div>
                      </div>

                      <div className="flex flex-col gap-2 pt-2">
                        <div className="text-xs font-medium text-foreground">
                          2. 售卖与交付状态处置
                        </div>
                        <div className="flex flex-wrap items-center gap-2">
                          <Button
                            type="button"
                            variant="secondary"
                            size="sm"
                            disabled={loading || saleState === 'sales_closed'}
                            onClick={() => {
                              setGovernanceReason('');
                              setGovernanceDialog('close_sales');
                            }}
                          >
                            停止新售（保留已购访问）
                          </Button>
                          <Button
                            type="button"
                            variant="destructive"
                            size="sm"
                            disabled={loading || deliveryState === 'withdrawn'}
                            onClick={() => {
                              setGovernanceReason('');
                              setGovernanceDialog('withdraw');
                            }}
                          >
                            撤回交付（全站下架）
                          </Button>
                        </div>
                      </div>
                    </>
                  )}
                </CardContent>
              </Card>
            </div>
          </div>
        </TabsContent>
      </Tabs>

      {/* 删除章节确认弹窗 */}
      <AlertDialog
        open={Boolean(deletingSectionId)}
        onOpenChange={(open) => {
          if (!open && !loading) {
            setDeletingSectionId(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除该章节？</AlertDialogTitle>
            <AlertDialogDescription>
              删除章节将同时移除该章节下的所有课时编排，此操作不可撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={loading}>取消</AlertDialogCancel>
            <AlertDialogAction
              disabled={loading}
              onClick={(e) => {
                e.preventDefault();
                void handleConfirmDeleteSection();
              }}
            >
              确认删除章节
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 删除课时确认弹窗 */}
      <AlertDialog
        open={Boolean(deletingLessonId)}
        onOpenChange={(open) => {
          if (!open && !loading) {
            setDeletingLessonId(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除该课时？</AlertDialogTitle>
            <AlertDialogDescription>
              删除后该课时将从当前章节大纲中移除。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={loading}>取消</AlertDialogCancel>
            <AlertDialogAction
              disabled={loading}
              onClick={(e) => {
                e.preventDefault();
                void handleConfirmDeleteLesson();
              }}
            >
              确认删除课时
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 提审 / 审核 / 停售 / 撤回 确认弹窗 */}
      <AlertDialog
        open={Boolean(governanceDialog)}
        onOpenChange={(open) => {
          if (!open && !loading) {
            setGovernanceDialog(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {governanceDialog === 'submit'
                ? '确认提交管理员审核'
                : governanceDialog === 'approve'
                  ? '确认审核通过并发布'
                  : governanceDialog === 'reject'
                    ? '退回课程草稿'
                    : governanceDialog === 'close_sales'
                      ? '确认停止课程新售'
                      : '确认撤回课程交付'}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {governanceDialog === 'submit'
                ? '提交后课程将进入待审核队列，管理员核对内容与定价后即可发布。'
                : governanceDialog === 'approve'
                  ? '审核通过后，课程最新内容将正式发布并对公开课程目录可见。'
                  : governanceDialog === 'reject'
                    ? '请填写具体整改意见，方便讲师针对性完善后重新提审。'
                    : governanceDialog === 'close_sales'
                      ? '停售后不再接受新学员报名或购买，但已获授权的老学员可继续学习。'
                      : '撤回交付后课程立即下架并终止访问，仅用于合规或严重故障处置。'}
            </AlertDialogDescription>
          </AlertDialogHeader>

          {governanceDialog && governanceDialog !== 'submit' ? (
            <Field>
              <FieldLabel htmlFor="editor-governance-reason">
                {governanceDialog === 'reject' ||
                governanceDialog === 'withdraw'
                  ? '处置原因（必填）'
                  : '审核备注（可选）'}
              </FieldLabel>
              <Textarea
                id="editor-governance-reason"
                rows={3}
                placeholder="输入审核意见或处置原因…"
                value={governanceReason}
                onChange={(e) => setGovernanceReason(e.target.value)}
              />
            </Field>
          ) : null}

          <AlertDialogFooter>
            <AlertDialogCancel disabled={loading}>取消</AlertDialogCancel>
            <AlertDialogAction
              disabled={
                loading ||
                ((governanceDialog === 'reject' ||
                  governanceDialog === 'withdraw') &&
                  !governanceReason.trim())
              }
              onClick={(e) => {
                e.preventDefault();
                void handleConfirmGovernance();
              }}
            >
              {loading ? <Spinner data-icon="inline-start" /> : null}
              确认执行
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
