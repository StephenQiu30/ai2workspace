'use client';

import {Layout} from '@/components/layout/layout';
import {Heading, Text} from '@/components/layout/typography';
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
import {TopicList} from './topic_list';
import {PostComposerSheet} from './post_composer_sheet';

export interface TopicDetailViewProps {
  user: SessionUser | null;
  topic: TopicSummary;
  topics: TopicSummary[];
  posts: Api.PostResponse[];
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
    <Layout flow="stack" className="w-full gap-8">
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link href="/community">社区首页</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>{topic.name}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <Layout
        as="header"
        flow="stack"
        className="gap-4 sm:flex-row sm:items-end sm:justify-between"
      >
        <Layout flow="stack" className="gap-2">
          <Layout flow="row" className="flex-wrap items-center gap-2">
            <Heading
              level={1}
              size="page"
              className="font-heading text-foreground"
            >
              {topic.name}
            </Heading>
            <Badge variant="secondary">共 {postsTotal} 篇讨论</Badge>
          </Layout>
          <Text as="p" variant="body" className="max-w-2xl">
            {topic.description}
          </Text>
        </Layout>

        <Layout flow="row" className="flex-wrap items-center gap-2">
          <Button type="button" onClick={() => setComposerOpen(true)}>
            <MessageSquarePlusIcon data-icon="inline-start" />
            在本话题发起讨论
          </Button>
          <Button variant="secondary" asChild>
            <Link href="/community">返回社区首页</Link>
          </Button>
        </Layout>
      </Layout>

      <Layout
        flow="grid"
        className="grid-cols-1 gap-10 xl:grid-cols-[minmax(0,1fr)_360px] xl:gap-16"
      >
        <Layout
          as="section"
          flow="stack"
          aria-label={`${topic.name}讨论列表`}
          className="min-w-0 gap-8"
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
                  <Link href="/community">浏览全部讨论</Link>
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
        </Layout>

        <Layout
          as="aside"
          flow="stack"
          aria-label="其他话题与入口"
          className="gap-6"
        >
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
              <Layout flow="row" className="items-center gap-2">
                <CompassIcon
                  className="size-4 text-foreground"
                  aria-hidden="true"
                />
                <CardTitle>全部话题</CardTitle>
              </Layout>
              <CardDescription>切换到其他主题浏览公开讨论</CardDescription>
            </CardHeader>
            <CardContent>
              <TopicList topics={topics} current={topic.slug} />
            </CardContent>
          </Card>
        </Layout>
      </Layout>

      <PostComposerSheet
        mode="create"
        open={composerOpen}
        onOpenChange={setComposerOpen}
        user={user}
        defaultTopic={topic.slug}
        returnPath={currentPath}
      />
    </Layout>
  );
}
