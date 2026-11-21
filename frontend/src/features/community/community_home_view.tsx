'use client';

import {useState} from 'react';
import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {
  AlertCircleIcon,
  ArrowRightIcon,
  BookOpenIcon,
  CompassIcon,
  MessageSquarePlusIcon,
  MessageSquareTextIcon,
  UserCheckIcon,
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
import {ToggleGroup, ToggleGroupItem} from '@/components/ui/toggle-group';
import type {SessionUser} from '@/lib/auth/roles';
import {
  buildLoginHref,
  COMMUNITY_TOPICS,
  formatPriceCny,
  getCourseCategoryLabel,
} from '@/lib/format';
import type {HomeCourseItem, TopicSummary} from './community_data';
import {CommunityPagination} from './community_pagination';
import {PostCard} from './post_card';
import {PostComposerSheet} from './post_composer_sheet';

export interface CommunityHomeViewProps {
  basePath: '/' | '/community';
  user: SessionUser | null;
  activeTopic: string;
  page: number;
  pageSize: number;
  courses: HomeCourseItem[];
  coursesError: string | null;
  posts: CommunityApi.PostResponse[];
  postsTotal: number;
  postsError: string | null;
  topics: TopicSummary[];
  myCourses: LearningApi.MyCourseResponse[];
  myCoursesError: string | null;
}

function buildFilterHref(
  basePath: '/' | '/community',
  topic: string,
  page: number,
): string {
  const params = new URLSearchParams();
  if (topic && topic !== 'all') {
    params.set('topic', topic);
  }
  if (page > 1) {
    params.set('page', String(page));
  }
  const query = params.toString();
  return query ? `${basePath}?${query}` : basePath;
}

function formatCourseUnitCount(course: HomeCourseItem): string {
  const sectionCount = course.sections?.length ?? 0;
  const lessonCount =
    typeof course.lessonCount === 'number'
      ? course.lessonCount
      : (course.sections?.reduce(
          (sum, sec) => sum + (sec.lessons?.length ?? 0),
          0,
        ) ?? 0);
  if (sectionCount > 0) {
    return `${sectionCount} 章 · ${lessonCount} 课时`;
  }
  return `${lessonCount} 课时`;
}

function computeProgressMetrics(item: LearningApi.MyCourseResponse): {
  percent: number;
  summaryText: string;
  continueHref: string;
  actionLabel: string;
} {
  const summary = item.progressSummary;
  const total = summary?.totalLessonCount ?? item.lessonCount ?? 0;
  const completed = summary?.completedLessonCount ?? 0;
  const watched = summary?.watchedLessonCount ?? 0;
  const effectiveDone = completed > 0 ? completed : watched;
  const percent =
    total > 0 ? Math.min(100, Math.round((effectiveDone / total) * 100)) : 0;

  let summaryText = '尚未开始学习';
  if (total > 0 && completed > 0) {
    summaryText = `已完成 ${completed} / ${total} 课时`;
  } else if (total > 0 && watched > 0) {
    summaryText = `已学习 ${watched} / ${total} 课时`;
  } else if (total > 0) {
    summaryText = `共 ${total} 课时`;
  }

  const courseId = item.id ?? '';
  const lastLessonId = summary?.lastLessonId?.trim();
  const continueHref =
    courseId && lastLessonId
      ? `/learn/${courseId}/${lastLessonId}`
      : courseId
        ? `/courses/${courseId}`
        : '/learning';
  const actionLabel = lastLessonId ? '继续学习' : '进入课程';

  return {percent, summaryText, continueHref, actionLabel};
}

export function CommunityHomeView({
  basePath,
  user,
  activeTopic,
  page,
  pageSize,
  courses,
  coursesError,
  posts,
  postsTotal,
  postsError,
  topics,
  myCourses,
  myCoursesError,
}: CommunityHomeViewProps) {
  const router = useRouter();
  const [composerOpen, setComposerOpen] = useState(false);

  const currentPath = buildFilterHref(basePath, activeTopic, page);
  const loginHref = buildLoginHref(currentPath);
  const registerHref = `${loginHref}${loginHref.includes('?') ? '&' : '?'}mode=register`;

  function handleTopicChange(nextTopic: string) {
    if (!nextTopic) {
      return;
    }
    router.push(buildFilterHref(basePath, nextTopic, 1));
  }

  return (
    <div className="mx-auto flex w-full max-w-[1792px] flex-col gap-10">
      <header className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-col gap-1.5">
          <h1 className="font-heading text-[28px] leading-[40px] font-medium text-foreground">
            学屿社区
          </h1>
          <p className="max-w-2xl text-sm leading-6 text-muted-foreground">
            先发现结构化课程，再围绕真实学习问题与实践经验持续交流。
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button type="button" onClick={() => setComposerOpen(true)}>
            <MessageSquarePlusIcon data-icon="inline-start" />
            发起讨论
          </Button>
        </div>
      </header>

      <div className="grid grid-cols-1 gap-10 xl:grid-cols-[minmax(0,1fr)_360px] xl:gap-16">
        <div className="flex min-w-0 flex-col gap-12">
          {/* 1. 精选课程区（课程在前，按设计 010 第 5.2.1 节） */}
          <section
            aria-labelledby="home-courses-heading"
            className="flex flex-col gap-5"
          >
            <div className="flex items-center justify-between gap-4">
              <div className="flex flex-col gap-0.5">
                <h2
                  id="home-courses-heading"
                  className="font-heading text-[20px] leading-[30px] font-medium text-foreground"
                >
                  精选课程
                </h2>
                <p className="text-sm leading-6 text-muted-foreground">
                  已审核发布的体系课程，支持公开目录查阅与按需学习
                </p>
              </div>
              <Button variant="ghost" size="sm" asChild>
                <Link href="/courses">
                  全部课程
                  <ArrowRightIcon data-icon="inline-end" />
                </Link>
              </Button>
            </div>

            {coursesError ? (
              <Alert variant="destructive">
                <AlertCircleIcon />
                <AlertTitle>课程列表加载失败</AlertTitle>
                <AlertDescription>{coursesError}</AlertDescription>
              </Alert>
            ) : courses.length === 0 ? (
              <Empty className="bg-canvas-soft py-10">
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    <BookOpenIcon />
                  </EmptyMedia>
                  <EmptyTitle>暂无已发布课程</EmptyTitle>
                  <EmptyDescription>
                    讲师与管理员发布的新课程将会第一时间展示在这里。
                  </EmptyDescription>
                </EmptyHeader>
                <EmptyContent>
                  <Button variant="secondary" size="sm" asChild>
                    <Link href="/courses">前往课程广场</Link>
                  </Button>
                </EmptyContent>
              </Empty>
            ) : (
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {courses.map((course) => {
                  const priceLabel = formatPriceCny(
                    course.priceAmountMinor,
                    course.isFree,
                  );
                  const instructorName =
                    course.authorNickname?.trim() || '学屿讲师';

                  return (
                    <Card
                      key={course.id}
                      className="justify-between transition-colors duration-120 hover:bg-control-surface"
                    >
                      <CardHeader className="gap-2.5">
                        <div className="flex flex-wrap items-center justify-between gap-2">
                          <Badge variant="secondary">
                            {getCourseCategoryLabel(course.category)}
                          </Badge>
                          <span className="text-xs leading-5 text-muted-foreground">
                            {formatCourseUnitCount(course)}
                          </span>
                        </div>
                        <CardTitle className="line-clamp-2">
                          <Link
                            href={`/courses/${course.id}`}
                            className="rounded-xs underline-offset-4 outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                          >
                            {course.title}
                          </Link>
                        </CardTitle>
                        <CardDescription className="line-clamp-2">
                          {course.description?.trim() ||
                            '点击查看完整课程目录与学习说明。'}
                        </CardDescription>
                      </CardHeader>

                      <CardContent className="pt-0">
                        <p className="text-xs leading-5 text-muted-foreground">
                          讲师：{instructorName}
                        </p>
                      </CardContent>

                      <CardFooter className="justify-between pt-0">
                        <span className="text-sm leading-6 font-medium text-foreground">
                          {priceLabel}
                        </span>
                        <Button variant="secondary" size="sm" asChild>
                          <Link href={`/courses/${course.id}`}>查看课程</Link>
                        </Button>
                      </CardFooter>
                    </Card>
                  );
                })}
              </div>
            )}
          </section>

          {/* 2. 社区讨论信息流（讨论紧随） */}
          <section
            aria-labelledby="home-feed-heading"
            className="flex flex-col gap-5"
          >
            <div className="flex flex-col gap-4">
              <div className="flex flex-wrap items-baseline justify-between gap-2">
                <h2
                  id="home-feed-heading"
                  className="font-heading text-[20px] leading-[30px] font-medium text-foreground"
                >
                  社区讨论
                </h2>
                <span className="text-xs leading-5 text-muted-foreground">
                  按发布时间倒序 · 共 {postsTotal} 篇讨论
                </span>
              </div>

              <div className="overflow-x-auto pb-1">
                <ToggleGroup
                  type="single"
                  value={activeTopic}
                  onValueChange={handleTopicChange}
                  aria-label="按话题筛选讨论"
                  spacing={2}
                  className="flex-wrap"
                >
                  {COMMUNITY_TOPICS.map((item) => (
                    <ToggleGroupItem
                      key={item.slug}
                      value={item.slug}
                      size="sm"
                      className="h-9 rounded-md bg-secondary px-3 text-[13px] font-medium text-secondary-foreground hover:bg-control-hover data-[state=on]:bg-primary data-[state=on]:text-primary-foreground md:h-8"
                    >
                      {item.label}
                    </ToggleGroupItem>
                  ))}
                </ToggleGroup>
              </div>
            </div>

            {postsError ? (
              <Alert variant="destructive">
                <AlertCircleIcon />
                <AlertTitle>讨论列表加载失败</AlertTitle>
                <AlertDescription>{postsError}</AlertDescription>
              </Alert>
            ) : posts.length === 0 ? (
              <Empty className="bg-canvas-soft py-12">
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    <MessageSquareTextIcon />
                  </EmptyMedia>
                  <EmptyTitle>
                    {activeTopic === 'all'
                      ? '还没有公开讨论'
                      : '当前话题暂无讨论'}
                  </EmptyTitle>
                  <EmptyDescription>
                    {activeTopic === 'all'
                      ? '发布第一篇学习复盘、课程问题或项目经验，开启社区讨论。'
                      : '可以切换到全部话题浏览其他内容，或直接发起本话题的第一篇讨论。'}
                  </EmptyDescription>
                </EmptyHeader>
                <EmptyContent className="flex-row justify-center gap-2">
                  {activeTopic !== 'all' ? (
                    <Button variant="secondary" size="sm" asChild>
                      <Link href={basePath}>查看全部话题</Link>
                    </Button>
                  ) : null}
                  <Button
                    type="button"
                    size="sm"
                    onClick={() => setComposerOpen(true)}
                  >
                    发起讨论
                  </Button>
                </EmptyContent>
              </Empty>
            ) : (
              <div className="flex flex-col gap-3">
                {posts.map((post) => (
                  <PostCard key={post.id} post={post} />
                ))}
                <CommunityPagination
                  currentPage={page}
                  totalItems={postsTotal}
                  pageSize={pageSize}
                  buildPageHref={(nextPage) =>
                    buildFilterHref(basePath, activeTopic, nextPage)
                  }
                />
              </div>
            )}
          </section>
        </div>

        {/* 右侧辅助列：登录态显示真实继续学习进度；游客仅显示登录注册引导 */}
        <aside aria-label="学习与话题导航" className="flex flex-col gap-6">
          {user ? (
            <Card>
              <CardHeader>
                <div className="flex items-center justify-between gap-2">
                  <CardTitle>继续学习</CardTitle>
                  <Button variant="ghost" size="xs" asChild>
                    <Link href="/learning">全部学习</Link>
                  </Button>
                </div>
                <CardDescription>
                  {user.nickname} 的在学课程与最近学习位置
                </CardDescription>
              </CardHeader>
              <CardContent className="flex flex-col gap-4">
                {myCoursesError ? (
                  <Alert>
                    <AlertCircleIcon />
                    <AlertTitle>学习进度暂不可用</AlertTitle>
                    <AlertDescription>{myCoursesError}</AlertDescription>
                  </Alert>
                ) : myCourses.length === 0 ? (
                  <div className="flex flex-col gap-3 rounded-lg bg-background p-4">
                    <p className="text-sm leading-6 text-body">
                      尚未加入任何课程，先去课程广场挑选感兴趣的内容。
                    </p>
                    <div>
                      <Button variant="secondary" size="sm" asChild>
                        <Link href="/courses">浏览课程广场</Link>
                      </Button>
                    </div>
                  </div>
                ) : (
                  myCourses.slice(0, 3).map((course) => {
                    const metrics = computeProgressMetrics(course);
                    return (
                      <div
                        key={course.id}
                        className="flex flex-col gap-2.5 rounded-lg bg-background p-4"
                      >
                        <div className="flex items-start justify-between gap-2">
                          <Link
                            href={metrics.continueHref}
                            className="line-clamp-2 text-sm leading-5 font-medium text-foreground underline-offset-4 outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                          >
                            {course.title}
                          </Link>
                          <span className="shrink-0 text-xs leading-5 text-muted-foreground">
                            {metrics.percent}%
                          </span>
                        </div>
                        <Progress
                          value={metrics.percent}
                          aria-label={`${course.title} 学习进度 ${metrics.percent}%`}
                        />
                        <div className="flex items-center justify-between gap-2">
                          <span className="text-xs leading-5 text-muted-foreground">
                            {metrics.summaryText}
                          </span>
                          <Button variant="secondary" size="xs" asChild>
                            <Link href={metrics.continueHref}>
                              {metrics.actionLabel}
                            </Link>
                          </Button>
                        </div>
                      </div>
                    );
                  })
                )}
              </CardContent>
            </Card>
          ) : (
            <Card>
              <CardHeader>
                <div className="flex items-center gap-2">
                  <UserCheckIcon
                    className="size-4 text-foreground"
                    aria-hidden="true"
                  />
                  <CardTitle>加入学屿社区</CardTitle>
                </div>
                <CardDescription>
                  登录后可阅读讨论完整正文与评论、收藏帖子，并同步你的个人课程学习进度。
                </CardDescription>
              </CardHeader>
              <CardFooter className="flex-col items-stretch gap-2 sm:flex-row">
                <Button asChild className="flex-1">
                  <Link href={loginHref}>登录账号</Link>
                </Button>
                <Button variant="secondary" asChild className="flex-1">
                  <Link href={registerHref}>注册新账号</Link>
                </Button>
              </CardFooter>
            </Card>
          )}

          <Card>
            <CardHeader>
              <div className="flex items-center gap-2">
                <CompassIcon
                  className="size-4 text-foreground"
                  aria-hidden="true"
                />
                <CardTitle>话题导航</CardTitle>
              </div>
              <CardDescription>
                按学习主题进入对应话题页，集中阅读相关讨论
              </CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-1.5">
              {topics.map((topic) => {
                const isActive = activeTopic === topic.slug;
                return (
                  <Link
                    key={topic.slug}
                    href={`/topics/${topic.slug}`}
                    className={`flex items-start justify-between gap-3 rounded-lg px-3 py-2.5 transition-colors duration-120 outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 ${
                      isActive
                        ? 'bg-control-hover text-foreground'
                        : 'hover:bg-background'
                    }`}
                  >
                    <div className="flex min-w-0 flex-col gap-0.5">
                      <span className="text-sm leading-5 font-medium text-foreground">
                        {topic.name}
                      </span>
                      <span className="line-clamp-1 text-xs leading-5 text-muted-foreground">
                        {topic.description}
                      </span>
                    </div>
                    <Badge variant="secondary" className="shrink-0">
                      {topic.postCount}
                    </Badge>
                  </Link>
                );
              })}
            </CardContent>
          </Card>
        </aside>
      </div>

      <PostComposerSheet
        mode="create"
        open={composerOpen}
        onOpenChange={setComposerOpen}
        user={user}
        defaultTopic={activeTopic === 'all' ? 'general' : activeTopic}
        returnPath={currentPath}
      />
    </div>
  );
}
