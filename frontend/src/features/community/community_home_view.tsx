'use client';

import {Layout} from '@/components/layout/layout';
import {Heading, Text} from '@/components/layout/typography';
import {useState} from 'react';
import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {
  AlertCircleIcon,
  BookOpenIcon,
  CompassIcon,
  MessageSquareTextIcon,
} from 'lucide-react';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {Button} from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
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
import {COMMUNITY_TOPICS, formatPriceCny} from '@/lib/format';
import type {HomeCourseItem, TopicSummary} from './community_data';
import {CommunityPagination} from './community_pagination';
import {PostCard} from './post_card';
import {TopicList} from './topic_list';
import {ServiceShowcase} from './service_showcase';
import {CourseCover} from '@/features/course/course_cover';
import {
  Item,
  ItemMedia,
  ItemContent,
  ItemDescription,
  ItemTitle,
} from '@/components/ui/item';
import {PostComposerSheet} from './post_composer_sheet';

export interface CommunityHomeViewProps {
  user: SessionUser | null;
  activeTopic: string;
  page: number;
  pageSize: number;
  courses: HomeCourseItem[];
  services: Api.PostResponse[];
  servicesError: string | null;
  coursesError: string | null;
  posts: Api.PostResponse[];
  postsTotal: number;
  postsError: string | null;
  topics: TopicSummary[];
  myCourses: Api.MyCourseResponse[];
  myCoursesError: string | null;
  composeTopic?: string;
  courseTitle?: string;
}

function buildFilterHref(topic: string, page: number): string {
  const params = new URLSearchParams();
  if (topic && topic !== 'all') {
    params.set('topic', topic);
  }
  if (page > 1) {
    params.set('page', String(page));
  }
  const query = params.toString();
  return query ? `/community?${query}` : '/community';
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

function computeProgressMetrics(item: Api.MyCourseResponse): {
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
  user,
  activeTopic,
  page,
  pageSize,
  courses,
  services,
  servicesError,
  coursesError,
  posts,
  postsTotal,
  postsError,
  topics,
  myCourses,
  myCoursesError,
  composeTopic,
  courseTitle,
}: CommunityHomeViewProps) {
  const router = useRouter();
  const [composerOpen, setComposerOpen] = useState(Boolean(composeTopic));

  const currentPath = buildFilterHref(activeTopic, page);

  function openComposer() {
    setComposerOpen(true);
    document
      .getElementById('community-composer')
      ?.scrollIntoView({block: 'start'});
  }

  function handleTopicChange(nextTopic: string) {
    if (!nextTopic) {
      return;
    }
    router.push(buildFilterHref(nextTopic, 1));
  }

  return (
    <Layout flow="stack" className="w-full gap-6">
      <Layout
        className={
          composerOpen
            ? 'grid'
            : 'grid gap-4 lg:grid-cols-[minmax(0,3fr)_minmax(0,1fr)]'
        }
      >
        {!composerOpen && (
          <ServiceShowcase services={services} error={servicesError} />
        )}
        <PostComposerSheet
          inline
          mode="create"
          open={composerOpen}
          onOpenChange={setComposerOpen}
          user={user}
          defaultTopic={
            composeTopic || (activeTopic === 'all' ? 'general' : activeTopic)
          }
          courseTitle={courseTitle}
          returnPath={currentPath}
          onSuccess={(post) => {
            router.push(
              buildFilterHref(
                activeTopic !== 'all' && post.topic ? post.topic : 'all',
                1,
              ),
            );
          }}
        />
      </Layout>
      <Layout
        flow="grid"
        className="grid-cols-1 items-start gap-4 lg:grid-cols-[minmax(0,3fr)_minmax(0,1fr)]"
      >
        <Layout
          as="section"
          flow="stack"
          aria-labelledby="home-feed-heading"
          className="gap-5"
        >
          <Layout flow="stack" className="gap-4">
            <Layout
              flow="row"
              className="flex-wrap items-baseline justify-between gap-2"
            >
              <Heading
                level={2}
                size="section"
                id="home-feed-heading"
                className="font-heading text-foreground"
              >
                最新讨论
              </Heading>
              <Text variant="meta">{!postsError && `${postsTotal} 篇`}</Text>
            </Layout>

            <Layout className="overflow-x-auto pb-1">
              <ToggleGroup
                type="single"
                value={activeTopic}
                onValueChange={handleTopicChange}
                aria-label="筛选社区内容"
                variant="filter"
                spacing={2}
                className="flex-wrap"
              >
                {COMMUNITY_TOPICS.filter((item) =>
                  [
                    'all',
                    'services',
                    'course-qa',
                    'project-practice',
                    activeTopic,
                  ].includes(item.slug),
                ).map((item) => (
                  <ToggleGroupItem
                    key={item.slug}
                    value={item.slug}
                    size="sm"
                    className="min-h-11 px-3 md:min-h-9"
                  >
                    {item.slug === 'all' ? '全部' : item.label}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            </Layout>
          </Layout>

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
                    ? '分享你的第一件作品或提出一个问题。'
                    : '可切换话题，或发布第一条内容。'}
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent className="flex-row justify-center gap-2">
                {activeTopic !== 'all' ? (
                  <Button variant="secondary" size="sm" asChild>
                    <Link href="/community">查看全部话题</Link>
                  </Button>
                ) : null}
                <Button type="button" size="sm" onClick={openComposer}>
                  发起讨论
                </Button>
              </EmptyContent>
            </Empty>
          ) : (
            <Layout flow="stack" className="gap-8">
              {posts.map((post) => (
                <PostCard key={post.id} post={post} />
              ))}
              <CommunityPagination
                currentPage={page}
                totalItems={postsTotal}
                pageSize={pageSize}
                buildPageHref={(nextPage) =>
                  buildFilterHref(activeTopic, nextPage)
                }
              />
            </Layout>
          )}
        </Layout>

        <Layout
          as="aside"
          flow="stack"
          aria-label="课程与话题"
          className="min-w-0 gap-6"
        >
          <Card surface="plain">
            <CardHeader>
              <CardTitle>发现课程</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              {coursesError ? (
                <Text as="p">课程暂时无法加载。</Text>
              ) : courses.length === 0 ? (
                <Text as="p">暂无已发布课程</Text>
              ) : (
                courses.map((course) => (
                  <Item
                    asChild
                    size="sm"
                    key={course.id}
                    className="gap-3 px-0"
                  >
                    <Link href={`/courses/${course.id}`}>
                      <ItemMedia
                        variant="image"
                        className="relative aspect-[3/2] h-auto w-16 group-data-[size=sm]/item:h-auto group-data-[size=sm]/item:w-16 shrink-0 items-center justify-center overflow-hidden rounded-md bg-muted"
                      >
                        <BookOpenIcon aria-hidden="true" className="size-4" />
                        <Layout className="absolute inset-0 [&_img]:size-full">
                          <CourseCover
                            category={course.category}
                            coverUrl={course.coverUrl}
                          />
                        </Layout>
                      </ItemMedia>
                      <ItemContent className="min-w-0">
                        <ItemTitle>{course.title}</ItemTitle>
                        <ItemDescription>
                          {formatCourseUnitCount(course)} ·{' '}
                          {formatPriceCny(
                            course.priceAmountMinor,
                            course.isFree,
                          )}
                        </ItemDescription>
                      </ItemContent>
                    </Link>
                  </Item>
                ))
              )}
              <Button variant="secondary" asChild>
                <Link href="/courses">全部课程</Link>
              </Button>
            </CardContent>
          </Card>
          {user ? (
            <Card surface="plain">
              <CardHeader>
                <Layout
                  flow="row"
                  className="items-center justify-between gap-2"
                >
                  <CardTitle>继续学习</CardTitle>
                  <Button variant="ghost" size="xs" asChild>
                    <Link href="/learning">全部学习</Link>
                  </Button>
                </Layout>
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
                  <Card className="flex flex-col gap-3 bg-background p-4">
                    <Text as="p" variant="body">
                      尚未加入任何课程，先去课程广场挑选感兴趣的内容。
                    </Text>
                    <Layout>
                      <Button variant="secondary" size="sm" asChild>
                        <Link href="/courses">浏览课程广场</Link>
                      </Button>
                    </Layout>
                  </Card>
                ) : (
                  myCourses.slice(0, 3).map((course) => {
                    const metrics = computeProgressMetrics(course);
                    return (
                      <Card
                        key={course.id}
                        className="flex flex-col gap-2.5 bg-background p-4"
                      >
                        <Layout
                          flow="row"
                          className="items-start justify-between gap-2"
                        >
                          <Link
                            href={metrics.continueHref}
                            className="line-clamp-2 text-sm leading-5 font-medium text-foreground underline-offset-4 outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                          >
                            {course.title}
                          </Link>
                          <Text variant="meta" className="shrink-0">
                            {metrics.percent}%
                          </Text>
                        </Layout>
                        <Progress
                          value={metrics.percent}
                          aria-label={`${course.title} 学习进度 ${metrics.percent}%`}
                        />
                        <Layout
                          flow="row"
                          className="items-center justify-between gap-2"
                        >
                          <Text variant="meta">{metrics.summaryText}</Text>
                          <Button variant="secondary" size="xs" asChild>
                            <Link href={metrics.continueHref}>
                              {metrics.actionLabel}
                            </Link>
                          </Button>
                        </Layout>
                      </Card>
                    );
                  })
                )}
              </CardContent>
            </Card>
          ) : null}

          <Card surface="plain">
            <CardHeader>
              <Layout flow="row" className="items-center gap-2">
                <CompassIcon
                  className="size-4 text-foreground"
                  aria-hidden="true"
                />
                <CardTitle>话题导航</CardTitle>
              </Layout>
            </CardHeader>
            <CardContent>
              <TopicList topics={topics} current={activeTopic} />
            </CardContent>
          </Card>
        </Layout>
      </Layout>
    </Layout>
  );
}
