'use client';

import {useState} from 'react';
import Link from 'next/link';
import {
  AlertCircleIcon,
  CompassIcon,
  MessageSquarePlusIcon,
  MessageSquareTextIcon,
} from 'lucide-react';
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
import type {SessionUser} from '@/lib/auth/roles';
import {buildLoginHref} from '@/lib/format';
import type {TopicSummary} from './community_data';
import {CommunityPagination} from './community_pagination';
import {PostCard} from './post_card';
import {PostComposerSheet} from './post_composer_sheet';

export interface TopicDetailViewProps {
  user: SessionUser | null;
  topic: TopicSummary;
  topics: TopicSummary[];
  posts: CommunityApi.PostResponse[];
  postsTotal: number;
  page: number;
  pageSize: number;
  postsError: string | null;
}

function buildTopicPageHref(slug: string, page: number): string {
  return page > 1 ? `/topics/${slug}?page=${page}` : `/topics/${slug}`;
}

export function TopicDetailView({
  user,
  topic,
  topics,
  posts,
  postsTotal,
  page,
  pageSize,
  postsError,
}: TopicDetailViewProps) {
  const [composerOpen, setComposerOpen] = useState(false);

  const currentPath = buildTopicPageHref(topic.slug, page);
  const loginHref = buildLoginHref(currentPath);

  return (
    <div className="mx-auto flex w-full max-w-[1792px] flex-col gap-8">
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link href="/">社区首页</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>{topic.name}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <header className="flex flex-col gap-4 rounded-xl bg-canvas-soft p-6 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-col gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="font-heading text-[28px] leading-[40px] font-medium text-foreground">
              {topic.name}
            </h1>
            <Badge variant="secondary">共 {postsTotal} 篇讨论</Badge>
          </div>
          <p className="max-w-2xl text-sm leading-6 text-body">
            {topic.description}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button type="button" onClick={() => setComposerOpen(true)}>
            <MessageSquarePlusIcon data-icon="inline-start" />
            在本话题发起讨论
          </Button>
          <Button variant="secondary" asChild>
            <Link href="/">返回首页</Link>
          </Button>
        </div>
      </header>

      <div className="grid grid-cols-1 gap-10 xl:grid-cols-[minmax(0,1fr)_360px] xl:gap-16">
        <section
          aria-label={`${topic.name}讨论列表`}
          className="flex min-w-0 flex-col gap-3"
        >
          {postsError ? (
            <Alert variant="destructive">
              <AlertCircleIcon />
              <AlertTitle>话题讨论加载失败</AlertTitle>
              <AlertDescription>{postsError}</AlertDescription>
            </Alert>
          ) : posts.length === 0 ? (
            <Empty className="bg-canvas-soft py-12">
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <MessageSquareTextIcon />
                </EmptyMedia>
                <EmptyTitle>该话题暂时还没有讨论</EmptyTitle>
                <EmptyDescription>
                  发布第一篇与「{topic.name}」相关的学习思考或实践问题。
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent className="flex-row justify-center gap-2">
                <Button variant="secondary" size="sm" asChild>
                  <Link href="/">浏览全部讨论</Link>
                </Button>
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
            <>
              {posts.map((post) => (
                <PostCard key={post.id} post={post} hideTopicLink />
              ))}
              <CommunityPagination
                currentPage={page}
                totalItems={postsTotal}
                pageSize={pageSize}
                buildPageHref={(nextPage) =>
                  buildTopicPageHref(topic.slug, nextPage)
                }
              />
            </>
          )}
        </section>

        <aside aria-label="其他话题与入口" className="flex flex-col gap-6">
          {!user ? (
            <Card>
              <CardHeader>
                <CardTitle>登录后阅读正文与互动</CardTitle>
                <CardDescription>
                  游客可浏览公开标题与独立摘要，登录后即可阅读完整讨论正文与评论。
                </CardDescription>
              </CardHeader>
              <CardFooter>
                <Button asChild className="w-full">
                  <Link href={loginHref}>登录账号</Link>
                </Button>
              </CardFooter>
            </Card>
          ) : null}

          <Card>
            <CardHeader>
              <div className="flex items-center gap-2">
                <CompassIcon
                  className="size-4 text-foreground"
                  aria-hidden="true"
                />
                <CardTitle>全部话题</CardTitle>
              </div>
              <CardDescription>切换到其他主题浏览公开讨论</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-1.5">
              {topics.map((item) => {
                const isCurrent = item.slug === topic.slug;
                return (
                  <Link
                    key={item.slug}
                    href={`/topics/${item.slug}`}
                    aria-current={isCurrent ? 'page' : undefined}
                    className={`flex items-start justify-between gap-3 rounded-lg px-3 py-2.5 transition-colors duration-120 outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 ${
                      isCurrent
                        ? 'bg-control-hover text-foreground'
                        : 'hover:bg-background'
                    }`}
                  >
                    <div className="flex min-w-0 flex-col gap-0.5">
                      <span className="text-sm leading-5 font-medium text-foreground">
                        {item.name}
                      </span>
                      <span className="line-clamp-1 text-xs leading-5 text-muted-foreground">
                        {item.description}
                      </span>
                    </div>
                    <Badge variant="secondary" className="shrink-0">
                      {item.postCount}
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
        defaultTopic={topic.slug}
        returnPath={currentPath}
      />
    </div>
  );
}
