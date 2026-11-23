'use client';

import {Layout} from '@/components/layout/layout';
import {Heading, Text} from '@/components/layout/typography';
import {useCallback, useEffect, useRef, useState} from 'react';
import Link from 'next/link';
import {CourseVideo} from '@/components/media/course_video';
import {
  AlertCircleIcon,
  ArrowLeftIcon,
  ArrowRightIcon,
  BookOpenIcon,
  FileTextIcon,
  ListTreeIcon,
  LockIcon,
  MessageSquareIcon,
  PlayCircleIcon,
  RefreshCwIcon,
  SparklesIcon,
} from 'lucide-react';
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
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
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet';
import {Spinner} from '@/components/ui/spinner';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import {TiptapRenderer} from '@/components/editor/tiptap_renderer';
import {useAssistantContext} from '@/features/assistant/assistant_context';
import {
  flattenCourseLessons,
  formatDurationSeconds,
  getHttpStatus,
  sortSectionsAndLessons,
} from '@/features/course/course_status';
import type {SessionUser} from '@/lib/auth/roles';
import {buildLoginHref, formatDateTime, getErrorMessage} from '@/lib/format';
import {learningAuthorizePlayback} from '@/services/generated/learning/media';
import {
  learningGetProgress,
  learningSaveProgress,
} from '@/services/generated/learning/progress';

export interface LessonPlayerProps {
  readonly courseId: string;
  readonly lessonId: string;
  readonly isTrial: boolean;
  readonly contentVersion?: number;
  readonly lessonTitle: string;
  readonly lessonType?: string;
  readonly lessonContent?: string;
  readonly course?: LearningApi.CourseResponse;
  readonly discussions?: CommunityApi.PostResponse[];
  readonly user?: SessionUser | null;
}

export function LessonPlayer({
  courseId,
  lessonId,
  isTrial,
  contentVersion = 1,
  lessonTitle,
  lessonType = 'video',
  lessonContent,
  course,
  discussions = [],
  user = null,
}: LessonPlayerProps) {
  const {openWithCourse} = useAssistantContext();

  const isVideo = lessonType !== 'article';
  const courseTitle = course?.title || '课程学习';
  const sortedSections = sortSectionsAndLessons(course?.sections);
  const allLessons = flattenCourseLessons(sortedSections);
  const currentFlatIndex = allLessons.findIndex(
    (item) => item.lesson.id === lessonId,
  );
  const currentFlat =
    currentFlatIndex >= 0 ? allLessons[currentFlatIndex] : undefined;
  const prevLesson =
    currentFlatIndex > 0 ? allLessons[currentFlatIndex - 1]?.lesson : undefined;
  const nextLesson =
    currentFlatIndex >= 0 && currentFlatIndex < allLessons.length - 1
      ? allLessons[currentFlatIndex + 1]?.lesson
      : undefined;

  const [playbackUrl, setPlaybackUrl] = useState<string | null>(null);
  const [qualities, setQualities] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(isVideo);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [accessDenied, setAccessDenied] = useState<boolean>(false);
  const [mediaNotReady, setMediaNotReady] = useState<boolean>(false);
  const [savedPosition, setSavedPosition] = useState<number>(0);
  const [progressSyncNote, setProgressSyncNote] = useState<string | null>(null);
  const [mobileOutlineOpen, setMobileOutlineOpen] = useState<boolean>(false);
  const [reloadNonce, setReloadNonce] = useState<number>(0);

  const videoRef = useRef<HTMLVideoElement | null>(null);
  const progressVersionRef = useRef<number>(0);
  const progressReportTimerRef = useRef<ReturnType<typeof setTimeout> | null>(
    null,
  );

  const loadProgress = useCallback(async () => {
    try {
      const prog = await learningGetProgress({id: lessonId});
      if (typeof prog.version === 'number') {
        progressVersionRef.current = prog.version;
      }
      if (prog.positionSeconds && prog.positionSeconds > 0) {
        setSavedPosition(prog.positionSeconds);
      }
    } catch {
      progressVersionRef.current = 0;
    }
  }, [lessonId]);

  const reportProgress = useCallback(
    async (positionSeconds: number) => {
      if (positionSeconds < 0) {
        return;
      }
      try {
        const resp = await learningSaveProgress(
          {id: lessonId},
          {
            courseId,
            contentVersion,
            positionSeconds: Math.floor(positionSeconds),
            expectedVersion: progressVersionRef.current,
          },
        );
        if (typeof resp.version === 'number') {
          progressVersionRef.current = resp.version;
        }
        setProgressSyncNote('学习进度已同步');
      } catch (err: unknown) {
        const status = getHttpStatus(err);
        if (status === 409) {
          // 乐观锁版本冲突时重新拉取最新进度版本
          await loadProgress();
        }
      }
    },
    [contentVersion, courseId, lessonId, loadProgress],
  );

  useEffect(() => {
    let isMounted = true;

    async function loadLessonData() {
      setErrorMessage(null);
      setAccessDenied(false);
      setMediaNotReady(false);
      setProgressSyncNote(null);

      if (!isVideo) {
        setIsLoading(false);
        await loadProgress();
        // 图文课时打开即记录一次学习进度
        if (user) {
          void reportProgress(1);
        }
        return;
      }

      setIsLoading(true);
      try {
        const auth = await learningAuthorizePlayback(
          {id: lessonId},
          {
            courseId,
            purpose: isTrial ? 'trial' : 'playback',
          },
        );

        if (!isMounted) {
          return;
        }

        if (!auth.playbackUrl) {
          setMediaNotReady(true);
        } else {
          setPlaybackUrl(auth.playbackUrl);
          setQualities(auth.hlsQualities ?? []);
        }

        await loadProgress();
      } catch (err: unknown) {
        if (!isMounted) {
          return;
        }
        const status = getHttpStatus(err);
        const msg = getErrorMessage(err, '获取课时播放授权失败');
        if (
          status === 401 ||
          status === 403 ||
          msg.includes('403') ||
          msg.toLowerCase().includes('denied') ||
          msg.toLowerCase().includes('access')
        ) {
          setAccessDenied(true);
        } else if (
          status === 404 ||
          status === 409 ||
          status === 422 ||
          msg.toLowerCase().includes('media') ||
          msg.includes('未发布') ||
          msg.includes('未就绪')
        ) {
          setMediaNotReady(true);
          setErrorMessage(msg);
        } else {
          setErrorMessage(msg);
        }
      } finally {
        if (isMounted) {
          setIsLoading(false);
        }
      }
    }

    void loadLessonData();

    return () => {
      isMounted = false;
      if (progressReportTimerRef.current) {
        clearTimeout(progressReportTimerRef.current);
        progressReportTimerRef.current = null;
      }
    };
  }, [
    courseId,
    isTrial,
    isVideo,
    lessonId,
    loadProgress,
    reloadNonce,
    reportProgress,
    user,
  ]);

  function handleTimeUpdate() {
    if (!videoRef.current) {
      return;
    }
    const current = videoRef.current.currentTime;
    if (current <= 0 || progressReportTimerRef.current) {
      return;
    }
    progressReportTimerRef.current = setTimeout(() => {
      progressReportTimerRef.current = null;
      const latest = videoRef.current?.currentTime ?? current;
      void reportProgress(latest);
    }, 5000);
  }

  function handleResumePosition() {
    if (videoRef.current && savedPosition > 0) {
      videoRef.current.currentTime = savedPosition;
      void videoRef.current.play();
    }
  }

  function handleOpenAssistant() {
    openWithCourse({
      courseId,
      courseTitle,
    });
  }

  const defaultOpenSections = sortedSections.map(
    (sec, idx) => sec.id ?? `sec-${idx}`,
  );

  function renderOutlineContent(onSelectLesson?: () => void) {
    if (sortedSections.length === 0) {
      return (
        <Text as="p" className="py-4 text-sm text-muted-foreground">
          当前课程暂无其他章节目录。
        </Text>
      );
    }

    return (
      <Accordion
        type="multiple"
        defaultValue={defaultOpenSections}
        className="flex flex-col gap-2.5"
      >
        {sortedSections.map((sec, sIdx) => {
          const secKey = sec.id ?? `sec-${sIdx}`;
          const lessons = sec.lessons ?? [];
          return (
            <AccordionItem
              key={secKey}
              value={secKey}
              className="rounded-xl bg-canvas-soft px-3.5 py-0.5 not-last:border-b-0"
            >
              <AccordionTrigger className="py-3 text-sm font-medium text-foreground hover:no-underline">
                <Text className="truncate pr-2 text-left">
                  第 {sIdx + 1} 章 · {sec.title}
                </Text>
              </AccordionTrigger>
              <AccordionContent className="pb-3">
                <Layout as="ul" flow="stack" className="gap-1">
                  {lessons.map((lsn, lIdx) => {
                    const isCurrent = lsn.id === lessonId;
                    const lsnIsVideo = lsn.type !== 'article';
                    return (
                      <Layout as="li" key={lsn.id ?? `${secKey}-${lIdx}`}>
                        <Link
                          href={`/learn/${courseId}/${lsn.id}`}
                          onClick={onSelectLesson}
                          aria-current={isCurrent ? 'page' : undefined}
                          className={`flex min-h-10 items-center justify-between gap-2 rounded-lg px-3 py-2 text-xs transition-colors duration-120 outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 ${
                            isCurrent
                              ? 'bg-primary font-medium text-primary-foreground'
                              : 'bg-background text-body hover:bg-secondary hover:text-foreground'
                          }`}
                        >
                          <Layout
                            flow="row"
                            className="min-w-0 items-center gap-2"
                          >
                            {lsnIsVideo ? (
                              <PlayCircleIcon className="size-3.5 shrink-0" />
                            ) : (
                              <FileTextIcon className="size-3.5 shrink-0" />
                            )}
                            <Text className="truncate">
                              {sIdx + 1}.{lIdx + 1} {lsn.title}
                            </Text>
                          </Layout>
                          {lsn.isTrial ? (
                            <Badge
                              variant={isCurrent ? 'secondary' : 'default'}
                            >
                              试看
                            </Badge>
                          ) : null}
                        </Link>
                      </Layout>
                    );
                  })}
                </Layout>
              </AccordionContent>
            </AccordionItem>
          );
        })}
      </Accordion>
    );
  }

  function renderStageArea() {
    if (accessDenied) {
      return (
        <Empty className="bg-canvas-soft py-14">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <LockIcon />
            </EmptyMedia>
            <EmptyTitle>需要课程学习权限后方可查看本课时</EmptyTitle>
            <EmptyDescription>
              本课时为受保护的正式学习内容。请先加入免费课程或完成课程购买；如您已退课或退款成功，对应访问权也会同步终止。
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Layout
              flow="row"
              className="flex-wrap items-center justify-center gap-3"
            >
              {!user ? (
                <Button asChild>
                  <Link href={buildLoginHref(`/learn/${courseId}/${lessonId}`)}>
                    登录账号
                  </Link>
                </Button>
              ) : null}
              <Button asChild variant={user ? 'default' : 'secondary'}>
                <Link href={`/courses/${courseId}`}>前往课程详情获取权限</Link>
              </Button>
            </Layout>
          </EmptyContent>
        </Empty>
      );
    }

    if (!isVideo) {
      const hasArticleText = Boolean(lessonContent && lessonContent.trim());
      return (
        <Card>
          <CardHeader className="gap-2">
            <Layout flow="row" className="flex-wrap items-center gap-2">
              <Badge variant="secondary">图文课时</Badge>
              {isTrial ? <Badge variant="default">免费试读</Badge> : null}
              {progressSyncNote ? (
                <Text className="text-xs text-muted-foreground">
                  {progressSyncNote}
                </Text>
              ) : null}
            </Layout>
            <CardTitle className="text-xl/7 font-medium">
              {lessonTitle}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {hasArticleText ? (
              <TiptapRenderer
                content={lessonContent}
                className="max-w-[760px]"
              />
            ) : (
              <Empty className="bg-canvas-soft py-12">
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    <FileTextIcon />
                  </EmptyMedia>
                  <EmptyTitle>本图文课时暂未填写正文</EmptyTitle>
                  <EmptyDescription>
                    讲师尚未发布此课时的图文讲义内容。
                  </EmptyDescription>
                </EmptyHeader>
              </Empty>
            )}
          </CardContent>
        </Card>
      );
    }

    return (
      <Layout flow="stack" className="gap-3">
        <Layout
          flow="row"
          className="relative aspect-video w-full items-center justify-center overflow-hidden rounded-xl bg-primary text-primary-foreground"
        >
          {isLoading ? (
            <Layout flow="stack" className="items-center gap-3 text-sm">
              <Spinner className="size-6" />
              <Text>正在校验课时权限并准备受保护媒体流…</Text>
            </Layout>
          ) : null}

          {!isLoading && mediaNotReady ? (
            <Layout
              flow="stack"
              className="max-w-md items-center gap-3 px-6 text-center"
            >
              <PlayCircleIcon className="size-8 opacity-80" />
              <Layout className="text-base font-medium">
                课时视频正在转码或尚未发布媒体版本
              </Layout>
              <Text as="p" className="text-xs leading-5 opacity-80">
                {errorMessage ||
                  '讲师上传的视频完成 H.264/HLS 转码并显式发布后，即可在此处正常播放。'}
              </Text>
              <Layout
                flow="row"
                className="flex-wrap items-center justify-center gap-2 pt-1"
              >
                <Button
                  type="button"
                  size="sm"
                  variant="secondary"
                  onClick={() => setReloadNonce((n) => n + 1)}
                >
                  <RefreshCwIcon data-icon="inline-start" />
                  重新检查
                </Button>
                <Button asChild size="sm" variant="secondary">
                  <Link href={`/courses/${courseId}`}>返回课程大纲</Link>
                </Button>
              </Layout>
            </Layout>
          ) : null}

          {!isLoading && !mediaNotReady && errorMessage ? (
            <Layout
              flow="stack"
              className="max-w-md items-center gap-3 px-6 text-center"
            >
              <AlertCircleIcon className="size-8 opacity-80" />
              <Layout className="text-base font-medium">视频加载失败</Layout>
              <Text as="p" className="text-xs leading-5 opacity-80">
                {errorMessage}
              </Text>
              <Button
                type="button"
                size="sm"
                variant="secondary"
                onClick={() => setReloadNonce((n) => n + 1)}
              >
                <RefreshCwIcon data-icon="inline-start" />
                重试加载
              </Button>
            </Layout>
          ) : null}

          {!isLoading && !mediaNotReady && !errorMessage && playbackUrl ? (
            <CourseVideo
              src={playbackUrl}
              videoRef={videoRef}
              resumeTime={
                savedPosition > 0
                  ? formatDurationSeconds(savedPosition)
                  : undefined
              }
              onTimeUpdate={handleTimeUpdate}
              onResume={handleResumePosition}
            />
          ) : null}
        </Layout>

        <Layout
          flow="row"
          className="flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground"
        >
          <Layout flow="row" className="flex-wrap items-center gap-2">
            <Badge variant="secondary">视频课时</Badge>
            {isTrial ? (
              <Badge variant="default">公开试看</Badge>
            ) : (
              <Badge variant="secondary">受控 HLS 播放</Badge>
            )}
            {qualities.map((q) => (
              <Badge key={q} variant="secondary">
                {q}
              </Badge>
            ))}
          </Layout>
          {progressSyncNote ? <Text>{progressSyncNote}</Text> : null}
        </Layout>
      </Layout>
    );
  }

  // 若独立被调用且未传入完整 course，退化为单卡片播放器以保持向后兼容
  if (!course) {
    return renderStageArea();
  }

  return (
    <Layout flow="stack" className="w-full gap-6">
      <Layout
        flow="stack"
        className="justify-between gap-4 sm:flex-row sm:items-center"
      >
        <Layout flow="stack" className="gap-1.5">
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>
                <BreadcrumbLink asChild>
                  <Link href="/courses">课程目录</Link>
                </BreadcrumbLink>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbLink asChild>
                  <Link href={`/courses/${courseId}`}>{courseTitle}</Link>
                </BreadcrumbLink>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbPage>{lessonTitle}</BreadcrumbPage>
              </BreadcrumbItem>
            </BreadcrumbList>
          </Breadcrumb>

          <Heading level={1} size="page" className="text-foreground">
            {currentFlat
              ? `${currentFlat.sectionIndex + 1}.${currentFlat.lessonIndex + 1} ${lessonTitle}`
              : lessonTitle}
          </Heading>
        </Layout>

        <Layout flow="row" className="flex-wrap items-center gap-2">
          <Sheet open={mobileOutlineOpen} onOpenChange={setMobileOutlineOpen}>
            <SheetTrigger asChild>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                className="lg:hidden"
              >
                <ListTreeIcon data-icon="inline-start" />
                章节目录
              </Button>
            </SheetTrigger>
            <SheetContent side="right" className="w-full sm:max-w-md">
              <SheetHeader>
                <SheetTitle>章节与课时目录</SheetTitle>
                <SheetDescription>{courseTitle}</SheetDescription>
              </SheetHeader>
              <Layout className="overflow-y-auto px-4 pb-6">
                {renderOutlineContent(() => setMobileOutlineOpen(false))}
              </Layout>
            </SheetContent>
          </Sheet>

          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={handleOpenAssistant}
          >
            <SparklesIcon data-icon="inline-start" />
            向学屿助手提问
          </Button>

          <Button asChild variant="ghost" size="sm">
            <Link href={`/courses/${courseId}`}>返回课程详情</Link>
          </Button>
        </Layout>
      </Layout>

      <Layout flow="grid" className="grid-cols-1 gap-8 lg:grid-cols-12">
        <Layout flow="stack" className="gap-6 lg:col-span-8">
          {renderStageArea()}

          <Card
            surface="muted"
            className="flex flex-wrap items-center justify-between gap-3 p-3.5"
          >
            {prevLesson?.id ? (
              <Button asChild variant="secondary" size="sm">
                <Link href={`/learn/${courseId}/${prevLesson.id}`}>
                  <ArrowLeftIcon data-icon="inline-start" />
                  上一课：{prevLesson.title}
                </Link>
              </Button>
            ) : (
              <Text className="text-xs text-muted-foreground">
                已是第一节课时
              </Text>
            )}

            {nextLesson?.id ? (
              <Button asChild variant="secondary" size="sm">
                <Link href={`/learn/${courseId}/${nextLesson.id}`}>
                  下一课：{nextLesson.title}
                  <ArrowRightIcon data-icon="inline-end" />
                </Link>
              </Button>
            ) : (
              <Text className="text-xs text-muted-foreground">
                已是最后一节课时
              </Text>
            )}
          </Card>

          <Tabs defaultValue="notes" className="flex flex-col gap-4">
            <Layout
              flow="row"
              className="flex-wrap items-center justify-between gap-2"
            >
              <TabsList>
                <TabsTrigger value="notes">本课说明</TabsTrigger>
                <TabsTrigger value="discussions">
                  课程讨论 ({discussions.length})
                </TabsTrigger>
              </TabsList>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={handleOpenAssistant}
              >
                <SparklesIcon data-icon="inline-start" />
                召唤 AI 助教解析本课
              </Button>
            </Layout>

            <TabsContent value="notes">
              <Card>
                <CardHeader>
                  <CardTitle>课时讲义与补充说明</CardTitle>
                  <CardDescription>
                    所属章节：{currentFlat?.sectionTitle || '课程正文'} · 版本 v
                    {contentVersion}
                  </CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-4">
                  {lessonContent && lessonContent.trim() ? (
                    <TiptapRenderer content={lessonContent} />
                  ) : (
                    <Text as="p" variant="body">
                      {course.description?.trim() ||
                        '本课时暂未附加额外图文讲义。遇到疑问可随时点击右上角“向学屿助手提问”，或在课程讨论区与讲师、同学交流。'}
                    </Text>
                  )}
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="discussions">
              <Card>
                <CardHeader className="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
                  <Layout flow="stack" className="gap-1">
                    <CardTitle>本课程讨论帖</CardTitle>
                    <CardDescription>
                      围绕《{courseTitle}》的答疑、笔记与实践经验交流
                    </CardDescription>
                  </Layout>
                  <Button asChild variant="secondary" size="sm">
                    <Link
                      href={`/community?scope=course&course_id=${encodeURIComponent(courseId)}`}
                    >
                      前往课程讨论区
                      <ArrowRightIcon data-icon="inline-end" />
                    </Link>
                  </Button>
                </CardHeader>
                <CardContent>
                  {discussions.length === 0 ? (
                    <Empty className="bg-canvas-soft py-10">
                      <EmptyHeader>
                        <EmptyMedia variant="icon">
                          <MessageSquareIcon />
                        </EmptyMedia>
                        <EmptyTitle>暂无课程讨论帖</EmptyTitle>
                        <EmptyDescription>
                          在学习过程中遇到问题或有心得体会？前往社区发起本课程的第一个讨论吧。
                        </EmptyDescription>
                      </EmptyHeader>
                      <EmptyContent>
                        <Button asChild variant="secondary" size="sm">
                          <Link
                            href={`/community?scope=course&course_id=${encodeURIComponent(courseId)}`}
                          >
                            去发起讨论
                          </Link>
                        </Button>
                      </EmptyContent>
                    </Empty>
                  ) : (
                    <Layout as="ul" flow="stack" className="gap-2.5">
                      {discussions.map((post, idx) => (
                        <Layout as="li" key={post.id ?? `post-${idx}`}>
                          <Link
                            href={
                              post.id ? `/community/${post.id}` : '/community'
                            }
                            className="flex flex-col gap-1.5 rounded-xl bg-canvas-soft p-4 transition-colors duration-120 outline-none hover:bg-secondary focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                          >
                            <Layout
                              flow="row"
                              className="items-center justify-between gap-2"
                            >
                              <Text className="text-sm font-medium text-foreground">
                                {post.title || '无标题讨论'}
                              </Text>
                              <Text className="shrink-0 text-xs text-muted-foreground">
                                {formatDateTime(post.createdAt)}
                              </Text>
                            </Layout>
                            {post.publicSummary ? (
                              <Text
                                as="p"
                                className="line-clamp-2 text-xs leading-5 text-body"
                              >
                                {post.publicSummary}
                              </Text>
                            ) : null}
                            <Layout className="text-xs text-muted-foreground">
                              作者：{post.authorNickname?.trim() || '学屿学员'}
                            </Layout>
                          </Link>
                        </Layout>
                      ))}
                    </Layout>
                  )}
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        </Layout>

        <Layout as="aside" className="hidden lg:col-span-4 lg:block">
          <Card>
            <CardHeader>
              <Layout flow="row" className="items-center justify-between gap-2">
                <CardTitle className="inline-flex items-center gap-2">
                  <BookOpenIcon className="size-4" />
                  <Text>课程目录</Text>
                </CardTitle>
                <Text className="text-xs text-muted-foreground">
                  共 {allLessons.length} 课时
                </Text>
              </Layout>
              <CardDescription className="line-clamp-1">
                {courseTitle}
              </CardDescription>
            </CardHeader>
            <CardContent>{renderOutlineContent()}</CardContent>
          </Card>
        </Layout>
      </Layout>

      {course.deliveryState === 'withdrawn' ? (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>课程已撤回交付</AlertTitle>
          <AlertDescription>
            本课程已被管理员撤回，课时播放已停用。
          </AlertDescription>
        </Alert>
      ) : null}
    </Layout>
  );
}
