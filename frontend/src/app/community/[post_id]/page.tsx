import {Layout} from '@/components/layout/layout';
import type {Metadata} from 'next';
import Link from 'next/link';
import {notFound} from 'next/navigation';
import {AlertCircleIcon, EyeOffIcon, Trash2Icon} from 'lucide-react';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb';
import {Button} from '@/components/ui/button';
import {Card, CardContent} from '@/components/ui/card';
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import {
  PostDetailView,
  type ReactionState,
} from '@/features/community/post_detail_view';
import {getServerSession} from '@/lib/auth/session';
import {getErrorMessage} from '@/lib/format';
import {getServerRequestOptions} from '@/lib/request/server';
import {
  communityGetFavoriteStatus,
  communityGetPost,
  communityGetReaction,
} from '@/services/generated/community/community';

export const dynamic = 'force-dynamic';

interface PostDetailPageProps {
  params: Promise<{
    post_id: string;
  }>;
}

interface HttpErrorShape {
  response?: {
    status?: number;
    data?: CommunityApi.PostUnavailableResponse;
  };
}

function classifyPostError(err: unknown): {
  kind: 'not_found' | 'deleted' | 'hidden' | 'other';
  message: string;
} {
  const httpErr = err as HttpErrorShape | undefined;
  const httpStatus = httpErr?.response?.status;
  const data = httpErr?.response?.data;
  const statusTag = (data?.status ?? data?.accessState ?? data?.error ?? '')
    .toLowerCase()
    .trim();
  const message = getErrorMessage(err, '讨论详情暂时无法加载，请稍后重试');

  if (httpStatus === 404 || statusTag === 'not_found') {
    return {kind: 'not_found', message};
  }
  if (httpStatus === 410 || statusTag === 'deleted' || statusTag === 'hidden') {
    if (statusTag.includes('hidden') || message.includes('隐藏')) {
      return {
        kind: 'hidden',
        message:
          data?.message?.trim() ||
          '该讨论因违反社区内容规范已被管理员隐藏，暂不可访问。',
      };
    }
    return {
      kind: 'deleted',
      message:
        data?.message?.trim() || '该讨论已由作者自行删除，不再对外展示。',
    };
  }
  return {kind: 'other', message};
}

export async function generateMetadata({
  params,
}: PostDetailPageProps): Promise<Metadata> {
  const {post_id: postId} = await params;
  try {
    const options = await getServerRequestOptions();
    const post = await communityGetPost({id: postId}, options);
    return {
      title: post.title?.trim() || '讨论详情',
      description:
        post.publicSummary?.trim() || '查看学屿社区公开讨论摘要与完整交流。',
    };
  } catch {
    return {
      title: '讨论详情',
    };
  }
}

export default async function PostDetailPage({params}: PostDetailPageProps) {
  const [{post_id: postId}, user] = await Promise.all([
    params,
    getServerSession(),
  ]);

  let options: Awaited<ReturnType<typeof getServerRequestOptions>>;
  try {
    options = await getServerRequestOptions();
  } catch (err: unknown) {
    return (
      <Layout flow="stack" className="w-full gap-6">
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>服务端配置不可用</AlertTitle>
          <AlertDescription>
            {getErrorMessage(err, 'API_ORIGIN 未配置')}
          </AlertDescription>
        </Alert>
      </Layout>
    );
  }

  let post: CommunityApi.PostDetailResponse;
  try {
    post = await communityGetPost({id: postId}, options);
  } catch (err: unknown) {
    const classified = classifyPostError(err);
    if (classified.kind === 'not_found') {
      notFound();
    }

    if (classified.kind === 'deleted' || classified.kind === 'hidden') {
      const isHidden = classified.kind === 'hidden';
      return (
        <Layout flow="stack" className="w-full gap-8">
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>
                <BreadcrumbLink asChild>
                  <Link href="/">社区首页</Link>
                </BreadcrumbLink>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbPage>
                  {isHidden ? '内容已隐藏' : '讨论已删除'}
                </BreadcrumbPage>
              </BreadcrumbItem>
            </BreadcrumbList>
          </Breadcrumb>

          <Card>
            <CardContent className="py-14">
              <Empty>
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    {isHidden ? <EyeOffIcon /> : <Trash2Icon />}
                  </EmptyMedia>
                  <EmptyTitle>
                    {isHidden ? '该讨论已被管理员隐藏' : '该讨论已被作者删除'}
                  </EmptyTitle>
                  <EmptyDescription>{classified.message}</EmptyDescription>
                </EmptyHeader>
                <EmptyContent className="flex-row justify-center gap-2">
                  <Button asChild>
                    <Link href="/">返回社区首页</Link>
                  </Button>
                  <Button variant="secondary" asChild>
                    <Link href="/courses">浏览精选课程</Link>
                  </Button>
                </EmptyContent>
              </Empty>
            </CardContent>
          </Card>
        </Layout>
      );
    }

    return (
      <Layout flow="stack" className="w-full gap-6">
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>无法加载讨论详情</AlertTitle>
          <AlertDescription>{classified.message}</AlertDescription>
        </Alert>
        <Layout>
          <Button variant="secondary" asChild>
            <Link href="/">返回社区首页</Link>
          </Button>
        </Layout>
      </Layout>
    );
  }

  // 若返回体本身携带 deleted / hidden 状态，同样区分展示
  if (
    post.status === 'deleted' ||
    post.accessState === 'deleted' ||
    post.status === 'hidden' ||
    post.accessState === 'hidden'
  ) {
    const isHidden = post.status === 'hidden' || post.accessState === 'hidden';
    return (
      <Layout flow="stack" className="w-full gap-8">
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbLink asChild>
                <Link href="/">社区首页</Link>
              </BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <BreadcrumbPage>
                {isHidden ? '内容已隐藏' : '讨论已删除'}
              </BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>

        <Card>
          <CardContent className="py-14">
            <Empty>
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  {isHidden ? <EyeOffIcon /> : <Trash2Icon />}
                </EmptyMedia>
                <EmptyTitle>
                  {isHidden ? '该讨论已被管理员隐藏' : '该讨论已被作者删除'}
                </EmptyTitle>
                <EmptyDescription>
                  {isHidden
                    ? '该讨论因违反社区内容规范已被管理员隐藏，暂不可访问。'
                    : '该讨论已由作者自行删除，不再对外展示。'}
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent className="flex-row justify-center gap-2">
                <Button asChild>
                  <Link href="/">返回社区首页</Link>
                </Button>
                <Button variant="secondary" asChild>
                  <Link href="/courses">浏览精选课程</Link>
                </Button>
              </EmptyContent>
            </Empty>
          </CardContent>
        </Card>
      </Layout>
    );
  }

  const canReadFull =
    post.accessState === 'full' ||
    (post.accessState !== 'login_required' &&
      post.accessState !== 'course_access_required' &&
      Boolean(post.body));

  let initialReaction: ReactionState = {
    likes: 0,
    dislikes: 0,
    myReaction: 'none',
  };
  let initialFavorited = false;
  const initialReactionsByCommentId: Record<string, ReactionState> = {};

  if (canReadFull) {
    const commentIds = (post.comments ?? [])
      .map((c) => c.id)
      .filter((id): id is string => Boolean(id));

    const [reactionResult, favoriteResult, commentReactionEntries] =
      await Promise.all([
        communityGetReaction({targetType: 'post', targetId: postId}, options)
          .then((res) => ({
            likes: res.likes ?? 0,
            dislikes: res.dislikes ?? 0,
            myReaction: res.myReaction ?? ('none' as const),
          }))
          .catch((): ReactionState => ({
            likes: 0,
            dislikes: 0,
            myReaction: 'none',
          })),
        user
          ? communityGetFavoriteStatus({postId}, options)
              .then((res) => Boolean(res.saved))
              .catch(() => false)
          : Promise.resolve(false),
        Promise.all(
          commentIds.map(async (cid) => {
            try {
              const res = await communityGetReaction(
                {targetType: 'comment', targetId: cid},
                options,
              );
              return [
                cid,
                {
                  likes: res.likes ?? 0,
                  dislikes: res.dislikes ?? 0,
                  myReaction: res.myReaction ?? ('none' as const),
                } satisfies ReactionState,
              ] as const;
            } catch {
              return [
                cid,
                {
                  likes: 0,
                  dislikes: 0,
                  myReaction: 'none',
                } satisfies ReactionState,
              ] as const;
            }
          }),
        ),
      ]);

    initialReaction = reactionResult;
    initialFavorited = favoriteResult;
    for (const [cid, state] of commentReactionEntries) {
      initialReactionsByCommentId[cid] = state;
    }
  }

  return (
    <PostDetailView
      post={post}
      user={user}
      initialReaction={initialReaction}
      initialFavorited={initialFavorited}
      initialReactionsByCommentId={initialReactionsByCommentId}
    />
  );
}
