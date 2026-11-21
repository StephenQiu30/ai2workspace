'use client';

import {useState} from 'react';
import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {
  AlertCircleIcon,
  ArrowRightIcon,
  BookOpenIcon,
  CheckCircle2Icon,
  FileTextIcon,
  InfoIcon,
  LockIcon,
  PlayCircleIcon,
  ShieldCheckIcon,
  ShoppingBagIcon,
  UserIcon,
} from 'lucide-react';
import {toast} from 'sonner';
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion';
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import {Spinner} from '@/components/ui/spinner';
import {
  countCourseLessons,
  flattenCourseLessons,
  sortSectionsAndLessons,
} from '@/features/course/course_status';
import type {SessionUser} from '@/lib/auth/roles';
import {
  buildLoginHref,
  formatPriceCny,
  getCourseCategoryLabel,
  getErrorMessage,
} from '@/lib/format';
import {commerceCreateOrder} from '@/services/generated/commerce/commerce';
import {learningJoinFreeCourse} from '@/services/generated/learning/courses';

export type CourseAccessSource = 'free' | 'purchased' | 'staff' | null;

export interface CourseDetailClientProps {
  readonly course: LearningApi.CourseResponse;
  readonly price: CommerceApi.PriceResponse | null;
  readonly user: SessionUser | null;
  readonly hasAccess: boolean;
  readonly accessSource: CourseAccessSource;
  readonly lastLessonId?: string | null;
}

function generateIdempotencyKey(courseId: string): string {
  if (
    typeof crypto !== 'undefined' &&
    typeof crypto.randomUUID === 'function'
  ) {
    return `order_${courseId}_${crypto.randomUUID()}`;
  }
  return `order_${courseId}_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

export function CourseDetailClient({
  course,
  price,
  user,
  hasAccess,
  accessSource,
  lastLessonId,
}: CourseDetailClientProps) {
  const router = useRouter();
  const courseId = course.id ?? '';
  const sortedSections = sortSectionsAndLessons(course.sections);
  const allLessons = flattenCourseLessons(sortedSections);
  const totalLessonCount = countCourseLessons(course);

  const firstLesson = allLessons[0]?.lesson ?? null;
  const firstTrialLesson =
    allLessons.find((item) => Boolean(item.lesson.isTrial))?.lesson ?? null;

  const resumeLessonId =
    lastLessonId && allLessons.some((item) => item.lesson.id === lastLessonId)
      ? lastLessonId
      : (firstLesson?.id ?? null);

  const isWithdrawn = course.deliveryState === 'withdrawn';
  const isSalesClosed = course.saleState === 'sales_closed';
  const isFree = Boolean(course.isFree);
  const hasValidPrice =
    typeof price?.amountMinor === 'number' && price.amountMinor > 0;

  const [joining, setJoining] = useState(false);
  const [buyDialogOpen, setBuyDialogOpen] = useState(false);
  const [creatingOrder, setCreatingOrder] = useState(false);
  const [orderError, setOrderError] = useState<string | null>(null);
  const [idempotencyKey, setIdempotencyKey] = useState<string>('');

  const defaultOpenSections = sortedSections.map(
    (section, index) => section.id ?? `section-${index}`,
  );

  function openPurchaseDialog() {
    setOrderError(null);
    setIdempotencyKey(generateIdempotencyKey(courseId));
    setBuyDialogOpen(true);
  }

  async function handleJoinFreeCourse() {
    if (!courseId) {
      return;
    }
    setJoining(true);
    try {
      await learningJoinFreeCourse({id: courseId});
      toast.success('已成功加入课程');
      if (firstLesson?.id) {
        router.push(`/learn/${courseId}/${firstLesson.id}`);
      } else {
        router.refresh();
      }
    } catch (err: unknown) {
      toast.error(getErrorMessage(err, '加入免费课程失败，请稍后重试'));
    } finally {
      setJoining(false);
    }
  }

  async function handleConfirmOrder() {
    if (!courseId) {
      return;
    }
    setCreatingOrder(true);
    setOrderError(null);
    const key = idempotencyKey || generateIdempotencyKey(courseId);
    if (!idempotencyKey) {
      setIdempotencyKey(key);
    }
    try {
      const order = await commerceCreateOrder({
        courseId,
        idempotencyKey: key,
      });
      if (!order.id) {
        throw new Error('订单已创建但未返回订单编号，请前往“我的订单”查看');
      }
      setBuyDialogOpen(false);
      toast.success('订单已创建，正在前往支付确认页');
      router.push(`/orders/${order.id}`);
    } catch (err: unknown) {
      const message = getErrorMessage(err, '创建订单失败，请稍后重试');
      setOrderError(message);
    } finally {
      setCreatingOrder(false);
    }
  }

  function renderAccessStatusBanner() {
    if (isWithdrawn) {
      return (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>课程暂不可用</AlertTitle>
          <AlertDescription>
            该课程因内容或合规原因已撤回交付，目前不可学习或购买。若您此前已付费购买，可前往“我的订单”查看售后与退款进度。
          </AlertDescription>
        </Alert>
      );
    }

    if (isSalesClosed && hasAccess) {
      return (
        <Alert>
          <InfoIcon />
          <AlertTitle>课程已停止新售</AlertTitle>
          <AlertDescription>
            该课程已关闭新报名与新购买入口。您当前已拥有学习权限，可继续正常学习全部已发布课时。
          </AlertDescription>
        </Alert>
      );
    }

    if (isSalesClosed && !hasAccess) {
      return (
        <Alert>
          <InfoIcon />
          <AlertTitle>该课程已停售</AlertTitle>
          <AlertDescription>
            当前课程已停止对外新购与新加入。如果您此前已经购买过本课程，请登录原购买账号继续学习。
          </AlertDescription>
        </Alert>
      );
    }

    return null;
  }

  function renderPrimaryCtaArea() {
    if (isWithdrawn) {
      return (
        <div className="flex flex-wrap items-center gap-3">
          <Button asChild variant="secondary">
            <Link href="/courses">浏览其他课程</Link>
          </Button>
          {user ? (
            <Button asChild variant="ghost">
              <Link href="/orders">查看我的订单</Link>
            </Button>
          ) : null}
        </div>
      );
    }

    // 1. 已登录且已有访问权（已报名/已购/归属讲师/管理员）
    if (user && hasAccess) {
      return (
        <div className="flex flex-col gap-3">
          <div className="flex items-center gap-2 text-sm text-body">
            <CheckCircle2Icon className="size-4 shrink-0 text-foreground" />
            <span>
              {accessSource === 'staff'
                ? '您以讲师或管理员身份拥有本课程完整访问权限'
                : accessSource === 'purchased'
                  ? '您已购买本课程，享有持续学习权益'
                  : '您已加入本免费课程'}
            </span>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            {resumeLessonId ? (
              <Button asChild size="lg">
                <Link href={`/learn/${courseId}/${resumeLessonId}`}>
                  {lastLessonId ? '继续学习' : '进入学习'}
                  <ArrowRightIcon data-icon="inline-end" />
                </Link>
              </Button>
            ) : (
              <Button type="button" size="lg" disabled>
                暂无可学习课时
              </Button>
            )}
            <Button asChild variant="secondary">
              <Link href="/learning">前往我的学习</Link>
            </Button>
          </div>
        </div>
      );
    }

    // 2. 游客
    if (!user) {
      const loginHref = buildLoginHref(`/courses/${courseId}`);
      return (
        <div className="flex flex-wrap items-center gap-3">
          <Button asChild size="lg">
            <Link href={loginHref}>
              {isSalesClosed
                ? '登录确认已有权益'
                : isFree
                  ? '登录并免费加入'
                  : '登录并购买课程'}
              <ArrowRightIcon data-icon="inline-end" />
            </Link>
          </Button>
          {firstTrialLesson?.id ? (
            <Button asChild variant="secondary" size="lg">
              <Link href={`/learn/${courseId}/${firstTrialLesson.id}`}>
                <PlayCircleIcon data-icon="inline-start" />
                立即试看
              </Link>
            </Button>
          ) : null}
        </div>
      );
    }

    // 3. 已登录但无访问权且已停售
    if (isSalesClosed) {
      return (
        <div className="flex flex-wrap items-center gap-3">
          <Button type="button" size="lg" disabled>
            课程已停售，暂不可新购
          </Button>
          {firstTrialLesson?.id ? (
            <Button asChild variant="secondary" size="lg">
              <Link href={`/learn/${courseId}/${firstTrialLesson.id}`}>
                <PlayCircleIcon data-icon="inline-start" />
                免费试看
              </Link>
            </Button>
          ) : null}
        </div>
      );
    }

    // 4. 已登录、免费课未加入
    if (isFree) {
      return (
        <div className="flex flex-wrap items-center gap-3">
          <Button
            type="button"
            size="lg"
            disabled={joining}
            onClick={() => void handleJoinFreeCourse()}
          >
            {joining ? <Spinner data-icon="inline-start" /> : null}
            {joining ? '正在加入…' : '免费加入学习'}
          </Button>
          {firstTrialLesson?.id ? (
            <Button asChild variant="secondary" size="lg">
              <Link href={`/learn/${courseId}/${firstTrialLesson.id}`}>
                <PlayCircleIcon data-icon="inline-start" />
                立即试看
              </Link>
            </Button>
          ) : null}
        </div>
      );
    }

    // 5. 已登录、付费课未购且在售
    return (
      <div className="flex flex-wrap items-center gap-3">
        <Button
          type="button"
          size="lg"
          disabled={!hasValidPrice}
          onClick={openPurchaseDialog}
        >
          <ShoppingBagIcon data-icon="inline-start" />
          {hasValidPrice
            ? `立即购买 · ${formatPriceCny(price?.amountMinor, false)}`
            : '课程暂未完成定价'}
        </Button>
        {firstTrialLesson?.id ? (
          <Button asChild variant="secondary" size="lg">
            <Link href={`/learn/${courseId}/${firstTrialLesson.id}`}>
              <PlayCircleIcon data-icon="inline-start" />
              立即试看
            </Link>
          </Button>
        ) : null}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-8">
      {renderAccessStatusBanner()}

      <div className="grid grid-cols-1 gap-8 lg:grid-cols-12">
        <div className="flex flex-col gap-5 lg:col-span-8">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="secondary">
              {getCourseCategoryLabel(course.category)}
            </Badge>
            <Badge variant={isFree ? 'secondary' : 'default'}>
              {isFree ? '免费公开课' : '人民币单课买断'}
            </Badge>
            {course.publishedVersion ? (
              <Badge variant="secondary">v{course.publishedVersion}</Badge>
            ) : null}
            {isSalesClosed ? <Badge variant="destructive">已停售</Badge> : null}
            {isWithdrawn ? <Badge variant="destructive">已撤回</Badge> : null}
          </div>

          <h1 className="text-[28px] leading-[40px] font-medium text-foreground">
            {course.title}
          </h1>

          <p className="max-w-[760px] text-base leading-7 text-body">
            {course.description?.trim() || '讲师暂未补充课程简介。'}
          </p>

          <div className="flex flex-wrap items-center gap-x-6 gap-y-2 text-sm text-muted-foreground">
            <span className="inline-flex items-center gap-1.5">
              <UserIcon className="size-4" />
              讲师：{course.authorNickname?.trim() || '学屿讲师'}
            </span>
            <span className="inline-flex items-center gap-1.5">
              <BookOpenIcon className="size-4" />共 {sortedSections.length} 章 ·{' '}
              {totalLessonCount} 课时
            </span>
          </div>

          <div className="pt-2">{renderPrimaryCtaArea()}</div>
        </div>

        <div className="lg:col-span-4">
          <Card>
            <CardHeader>
              <CardDescription>课程售价与学习权益</CardDescription>
              <CardTitle className="text-[28px] leading-[36px] font-medium">
                {formatPriceCny(price?.amountMinor, isFree)}
              </CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-3 text-sm leading-6 text-body">
              <div className="flex items-start gap-2.5">
                <ShieldCheckIcon className="mt-1 size-4 shrink-0 text-foreground" />
                <span>
                  {isFree
                    ? '免费公开课程，登录加入后即可记录跨设备学习进度并参与课程讨论。'
                    : '单课一次性买断（CNY），支付成功后自动激活本课程全部课时及专属讨论区权限。'}
                </span>
              </div>
              {!isFree ? (
                <div className="rounded-lg bg-secondary p-3 text-xs leading-5 text-body">
                  7×24 小时退款保障：自课程首次可交付起 7×24
                  小时内支持申请原路全额退款；超出窗口期的特殊情况可提交人工审核。
                </div>
              ) : null}
            </CardContent>
          </Card>
        </div>
      </div>

      <section className="flex flex-col gap-4">
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <h2 className="text-[20px] leading-[30px] font-medium text-foreground">
            课程大纲
          </h2>
          <span className="text-sm text-muted-foreground">
            {firstTrialLesson
              ? '标注“试看”的课时可直接进入体验'
              : hasAccess
                ? '点击任意课时即可开始学习'
                : '加入或购买课程后可解锁全部课时'}
          </span>
        </div>

        {sortedSections.length === 0 ? (
          <Empty className="bg-canvas-soft py-12">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <BookOpenIcon />
              </EmptyMedia>
              <EmptyTitle>课程大纲正在整理中</EmptyTitle>
              <EmptyDescription>
                讲师尚未发布章节与课时内容，请稍后再回来查看。
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <Accordion
            type="multiple"
            defaultValue={defaultOpenSections}
            className="flex flex-col gap-3"
          >
            {sortedSections.map((section, sectionIndex) => {
              const sectionKey = section.id ?? `section-${sectionIndex}`;
              const lessons = section.lessons ?? [];

              return (
                <AccordionItem
                  key={sectionKey}
                  value={sectionKey}
                  className="rounded-xl bg-canvas-soft px-5 py-1 not-last:border-b-0"
                >
                  <AccordionTrigger className="py-3.5 text-base font-medium text-foreground hover:no-underline">
                    <div className="flex flex-wrap items-center gap-2 pr-4">
                      <span>
                        第 {sectionIndex + 1} 章 · {section.title}
                      </span>
                      <span className="text-xs font-normal text-muted-foreground">
                        （{lessons.length} 课时）
                      </span>
                    </div>
                  </AccordionTrigger>
                  <AccordionContent className="pb-4">
                    {lessons.length === 0 ? (
                      <p className="py-2 text-sm text-muted-foreground">
                        本章节暂未添加课时。
                      </p>
                    ) : (
                      <ul className="flex flex-col gap-1.5">
                        {lessons.map((lesson, lessonIndex) => {
                          const canOpen =
                            !isWithdrawn &&
                            Boolean(lesson.id) &&
                            (hasAccess || Boolean(lesson.isTrial));
                          const isVideo = lesson.type === 'video';
                          const rowContent = (
                            <>
                              <div className="flex min-w-0 items-center gap-3">
                                {isVideo ? (
                                  <PlayCircleIcon className="size-4 shrink-0 text-foreground" />
                                ) : (
                                  <FileTextIcon className="size-4 shrink-0 text-foreground" />
                                )}
                                <span className="truncate text-sm font-medium text-foreground">
                                  {sectionIndex + 1}.{lessonIndex + 1}{' '}
                                  {lesson.title}
                                </span>
                              </div>
                              <div className="flex shrink-0 items-center gap-2">
                                <Badge variant="secondary">
                                  {isVideo ? '视频' : '图文'}
                                </Badge>
                                {lesson.isTrial ? (
                                  <Badge variant="default">试看</Badge>
                                ) : !hasAccess && !isFree ? (
                                  <Badge variant="secondary">付费</Badge>
                                ) : null}
                                {canOpen ? (
                                  <span className="inline-flex items-center gap-1 text-xs font-medium text-foreground">
                                    {lesson.isTrial && !hasAccess
                                      ? '试看'
                                      : '去学习'}
                                    <ArrowRightIcon className="size-3.5" />
                                  </span>
                                ) : (
                                  <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                                    <LockIcon className="size-3.5" />
                                    {isWithdrawn ? '不可用' : '加入后可学'}
                                  </span>
                                )}
                              </div>
                            </>
                          );

                          return (
                            <li
                              key={lesson.id ?? `${sectionKey}-${lessonIndex}`}
                            >
                              {canOpen && lesson.id ? (
                                <Link
                                  href={`/learn/${courseId}/${lesson.id}`}
                                  className="flex min-h-11 items-center justify-between gap-3 rounded-lg bg-background px-3.5 py-2.5 transition-colors duration-120 outline-none hover:bg-secondary focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                                >
                                  {rowContent}
                                </Link>
                              ) : (
                                <div className="flex min-h-11 items-center justify-between gap-3 rounded-lg bg-background/60 px-3.5 py-2.5 opacity-80">
                                  {rowContent}
                                </div>
                              )}
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
      </section>

      <Dialog open={buyDialogOpen} onOpenChange={setBuyDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认购买课程</DialogTitle>
            <DialogDescription>
              创建订单后将前往订单详情页完成支付确认，所有金额以服务端定价为准。
            </DialogDescription>
          </DialogHeader>

          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2 rounded-xl bg-canvas-soft p-4">
              <div className="text-sm font-medium text-foreground">
                {course.title}
              </div>
              <div className="flex items-center justify-between text-sm text-muted-foreground">
                <span>购买方式</span>
                <span className="text-foreground">人民币单课买断（CNY）</span>
              </div>
              <div className="flex items-center justify-between text-sm text-muted-foreground">
                <span>应付金额</span>
                <span className="text-lg font-medium text-foreground">
                  {formatPriceCny(price?.amountMinor, false)}
                </span>
              </div>
            </div>

            <div className="flex flex-col gap-1.5 rounded-xl bg-secondary p-4 text-xs leading-5 text-body">
              <div className="font-medium text-foreground">
                购买与 7×24 小时退款政策说明
              </div>
              <p>
                1.
                支付经服务端确认成功后，系统将为您开通本课程的全部已发布课时与课程讨论区权限。
              </p>
              <p>
                2. 自课程首次可交付起 7×24
                小时内，可随时在“我的订单”中发起全额退款；退款审核与渠道处理完成前保留学习权限，仅退款成功后撤销对应权益。
              </p>
              <p>
                3. 若超出 7×24
                小时但存在交付不可用、重复扣款等情况，可通过订单详情页的窗口外人工审核入口申请处理。
              </p>
            </div>

            {orderError ? (
              <Alert variant="destructive">
                <AlertCircleIcon />
                <AlertTitle>无法创建订单</AlertTitle>
                <AlertDescription>{orderError}</AlertDescription>
              </Alert>
            ) : null}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="secondary"
              disabled={creatingOrder}
              onClick={() => setBuyDialogOpen(false)}
            >
              取消
            </Button>
            <Button
              type="button"
              disabled={creatingOrder}
              onClick={() => void handleConfirmOrder()}
            >
              {creatingOrder ? <Spinner data-icon="inline-start" /> : null}
              {creatingOrder ? '正在创建订单…' : '确认下单并前往支付'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
