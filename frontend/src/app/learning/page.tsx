import {Layout} from '@/components/layout/layout';
import {Heading, Text} from '@/components/layout/typography';
import type {Metadata} from 'next';
import Link from 'next/link';
import {
  AlertCircleIcon,
  ArrowRightIcon,
  BookOpenIcon,
  ClockIcon,
  GraduationCapIcon,
  LockIcon,
  PlayCircleIcon,
  UserIcon,
} from 'lucide-react';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import {Progress} from '@/components/ui/progress';
import {
  countCourseLessons,
  flattenCourseLessons,
  formatDurationSeconds,
} from '@/features/course/course_status';
import {getServerSession} from '@/lib/auth/session';
import {
  buildLoginHref,
  formatDateTime,
  getCourseCategoryLabel,
} from '@/lib/format';
import {getServerRequestOptions} from '@/lib/request/server';
import {learningListMyCourses} from '@/services/generated/api/courses';

export const metadata: Metadata = {
  title: '我的学习',
  description: '查看您已加入或已购买的课程列表、学习进度与上次学习课时。',
};

export const dynamic = 'force-dynamic';

function resolveResumeLesson(course: Api.MyCourseResponse): {
  lessonId: string | null;
  lessonTitle: string | null;
  isResume: boolean;
} {
  const allLessons = flattenCourseLessons(course.sections);
  const lastId = course.progressSummary?.lastLessonId;
  if (lastId) {
    const matched = allLessons.find((item) => item.lesson.id === lastId);
    if (matched && matched.lesson.id) {
      return {
        lessonId: matched.lesson.id,
        lessonTitle: matched.lesson.title ?? null,
        isResume: true,
      };
    }
    return {
      lessonId: lastId,
      lessonTitle: null,
      isResume: true,
    };
  }
  const first = allLessons[0]?.lesson;
  if (first?.id) {
    return {
      lessonId: first.id,
      lessonTitle: first.title ?? null,
      isResume: false,
    };
  }
  return {lessonId: null, lessonTitle: null, isResume: false};
}

export default async function MyLearningPage() {
  const user = await getServerSession();

  if (!user) {
    return (
      <Layout flow="stack" className="w-full gap-6 py-8">
        <Empty className="bg-canvas-soft py-16">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <LockIcon />
            </EmptyMedia>
            <EmptyTitle>登录后查看我的学习</EmptyTitle>
            <EmptyDescription>
              登录账号后可同步您已免费加入或付费购买的课程，并从上次停留的位置继续学习。
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Layout
              flow="row"
              className="flex-wrap items-center justify-center gap-3"
            >
              <Button asChild>
                <Link href={buildLoginHref('/learning')}>前往登录</Link>
              </Button>
              <Button asChild variant="secondary">
                <Link href="/courses">先逛逛课程目录</Link>
              </Button>
            </Layout>
          </EmptyContent>
        </Empty>
      </Layout>
    );
  }

  let courses: Api.MyCourseResponse[] = [];
  let loadFailed = false;

  try {
    const options = await getServerRequestOptions();
    const list = await learningListMyCourses(options);
    courses = Array.isArray(list) ? list : [];
  } catch {
    loadFailed = true;
  }

  return (
    <Layout flow="stack" className="w-full gap-8">
      <Layout
        as="section"
        flow="stack"
        className="justify-between gap-4 sm:flex-row sm:items-end"
      >
        <Layout flow="stack" className="gap-2">
          <Heading level={1} size="page" className="text-foreground">
            我的学习
          </Heading>
          <Text as="p" variant="body">
            汇总您已报名的免费课程与已购买的买断课程，随时从上次学习位置继续。
          </Text>
        </Layout>
        <Layout flow="row" className="flex-wrap items-center gap-2">
          <Button asChild variant="secondary" size="sm">
            <Link href="/orders">查看我的订单</Link>
          </Button>
          <Button asChild variant="secondary" size="sm">
            <Link href="/courses">探索更多课程</Link>
          </Button>
        </Layout>
      </Layout>

      {loadFailed ? (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>无法加载学习记录</AlertTitle>
          <AlertDescription>
            学习服务暂时不可用，请刷新页面或稍后重试。
          </AlertDescription>
        </Alert>
      ) : null}

      {!loadFailed && courses.length === 0 ? (
        <Empty className="bg-canvas-soft py-16">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <GraduationCapIcon />
            </EmptyMedia>
            <EmptyTitle>还没有加入任何课程</EmptyTitle>
            <EmptyDescription>
              前往课程目录挑选感兴趣的免费公开课或买断课程，加入后即可在此追踪学习进度。
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button asChild>
              <Link href="/courses">
                前往课程目录
                <ArrowRightIcon data-icon="inline-end" />
              </Link>
            </Button>
          </EmptyContent>
        </Empty>
      ) : null}

      {courses.length > 0 ? (
        <Layout
          flow="grid"
          className="grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3"
        >
          {courses.map((course, index) => {
            const courseId = course.id ?? `course-${index}`;
            const summary = course.progressSummary;
            const totalLessons =
              summary?.totalLessonCount ?? countCourseLessons(course);
            const watchedLessons =
              summary?.watchedLessonCount ?? summary?.completedLessonCount ?? 0;
            const percent =
              totalLessons > 0
                ? Math.min(
                    100,
                    Math.round((watchedLessons / totalLessons) * 100),
                  )
                : 0;
            const resume = resolveResumeLesson(course);
            const isWithdrawn = course.deliveryState === 'withdrawn';
            const isSalesClosed = course.saleState === 'sales_closed';
            const isPurchased =
              course.enrollmentType === 'purchased' || !course.isFree;

            return (
              <Card
                key={courseId}
                className="flex h-full flex-col justify-between transition-colors duration-120 hover:bg-secondary/50"
              >
                <CardHeader className="gap-3">
                  <Layout flow="row" className="flex-wrap items-center gap-2">
                    <Badge variant="secondary">
                      {getCourseCategoryLabel(course.category)}
                    </Badge>
                    <Badge variant={isPurchased ? 'default' : 'secondary'}>
                      {isPurchased ? '已购课程' : '免费加入'}
                    </Badge>
                    {isSalesClosed ? (
                      <Badge variant="secondary">已停售·可继续学</Badge>
                    ) : null}
                    {isWithdrawn ? (
                      <Badge variant="destructive">已撤回</Badge>
                    ) : null}
                  </Layout>
                  <CardTitle className="line-clamp-2">
                    <Link
                      href={`/courses/${courseId}`}
                      className="outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                    >
                      {course.title || '未命名课程'}
                    </Link>
                  </CardTitle>
                  <CardDescription className="line-clamp-2">
                    {course.description?.trim() || '暂无课程简介'}
                  </CardDescription>
                </CardHeader>

                <CardContent className="flex flex-col gap-4">
                  <Layout
                    flow="row"
                    className="flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground"
                  >
                    <Text className="inline-flex items-center gap-1">
                      <UserIcon className="size-3.5" />
                      {course.authorNickname?.trim() || '学屿讲师'}
                    </Text>
                    <Text className="inline-flex items-center gap-1">
                      <BookOpenIcon className="size-3.5" />
                      已学 {watchedLessons} / {totalLessons} 课时
                    </Text>
                  </Layout>

                  <Layout flow="stack" className="gap-1.5">
                    <Layout
                      flow="row"
                      className="items-center justify-between text-xs"
                    >
                      <Text className="text-muted-foreground">学习进度</Text>
                      <Text className="font-medium text-foreground">
                        {percent}%
                      </Text>
                    </Layout>
                    <Progress
                      value={percent}
                      aria-label={`${course.title ?? '课程'}学习进度`}
                    />
                  </Layout>

                  <Card className="flex flex-col gap-1 bg-secondary p-3 text-xs text-body">
                    {resume.isResume ? (
                      <>
                        <Layout
                          flow="row"
                          className="items-center gap-1.5 font-medium text-foreground"
                        >
                          <PlayCircleIcon className="size-3.5 shrink-0" />
                          <Text className="truncate">
                            上次学习：
                            {resume.lessonTitle || '已记录课时位置'}
                          </Text>
                        </Layout>
                        <Layout
                          flow="row"
                          className="flex-wrap items-center gap-x-3 gap-y-1 text-muted-foreground"
                        >
                          {summary?.lastPositionSeconds &&
                          summary.lastPositionSeconds > 0 ? (
                            <Text className="inline-flex items-center gap-1">
                              <ClockIcon className="size-3" />
                              停留于{' '}
                              {formatDurationSeconds(
                                summary.lastPositionSeconds,
                              )}
                            </Text>
                          ) : null}
                          {summary?.lastStudiedAt ? (
                            <Text>
                              最近学习 {formatDateTime(summary.lastStudiedAt)}
                            </Text>
                          ) : null}
                        </Layout>
                      </>
                    ) : (
                      <Text className="text-muted-foreground">
                        尚未开始学习，点击下方按钮从第一节课时开始。
                      </Text>
                    )}
                  </Card>
                </CardContent>

                <CardFooter className="mt-auto items-center justify-between gap-2 pt-1">
                  <Button asChild variant="ghost" size="sm">
                    <Link href={`/courses/${courseId}`}>课程大纲</Link>
                  </Button>
                  {!isWithdrawn && resume.lessonId && course.id ? (
                    <Button asChild size="sm">
                      <Link href={`/learn/${course.id}/${resume.lessonId}`}>
                        {resume.isResume ? '继续学习' : '开始学习'}
                        <ArrowRightIcon data-icon="inline-end" />
                      </Link>
                    </Button>
                  ) : (
                    <Button asChild variant="secondary" size="sm">
                      <Link href={`/courses/${courseId}`}>查看详情</Link>
                    </Button>
                  )}
                </CardFooter>
              </Card>
            );
          })}
        </Layout>
      ) : null}
    </Layout>
  );
}
