import {Layout} from '@/components/layout/layout';
import {Text} from '@/components/layout/typography';
import Link from 'next/link';
import {ArrowUpRightIcon} from 'lucide-react';
import {
  Card,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {Button} from '@/components/ui/button';
import {Badge} from '@/components/ui/badge';
import {formatDateTime, getTopicMeta} from '@/lib/format';

export interface PostCardProps {
  post: CommunityApi.PostResponse;
  hideTopicLink?: boolean;
}

export function PostCard({post, hideTopicLink = false}: PostCardProps) {
  const topicMeta = getTopicMeta(post.topic);
  const authorLabel = post.authorNickname?.trim() || '学屿成员';

  return (
    <Layout as="article">
      <Card surface="plain" className="gap-2">
        <CardHeader className="gap-2">
          <Layout flow="row" className="flex-wrap items-center gap-2">
            {hideTopicLink ? (
              <Badge variant="secondary">{topicMeta.label}</Badge>
            ) : (
              <Badge variant="secondary" asChild>
                <Link href={`/topics/${topicMeta.slug}`}>
                  {topicMeta.label}
                </Link>
              </Badge>
            )}
            {post.scope === 'course' ? (
              <Badge variant="secondary">课程讨论</Badge>
            ) : null}
            <Text variant="meta">{authorLabel}</Text>
            <Text variant="meta" aria-hidden="true">
              ·
            </Text>
            <Text as="time" variant="meta" dateTime={post.createdAt}>
              {formatDateTime(post.createdAt)}
            </Text>
          </Layout>

          <CardTitle as="h3" className="text-[15px]/6">
            <Link
              href={`/community/${post.id}`}
              className="line-clamp-2 rounded-xs underline-offset-4 outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
            >
              {post.title}
            </Link>
          </CardTitle>

          <CardDescription className="line-clamp-3 text-body">
            {post.publicSummary?.trim() || '作者未提供补充摘要。'}
          </CardDescription>
        </CardHeader>

        <CardFooter>
          <Button asChild variant="link" size="sm" className="px-0">
            <Link href={`/community/${post.id}`}>
              {post.topic === 'services'
                ? '查看服务说明'
                : post.topic === 'course-qa'
                  ? '查看咨询'
                  : '查看讨论'}
              <ArrowUpRightIcon data-icon="inline-end" />
            </Link>
          </Button>
        </CardFooter>
      </Card>
    </Layout>
  );
}
