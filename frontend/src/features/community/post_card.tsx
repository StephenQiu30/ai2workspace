import Link from 'next/link';
import {ArrowUpRightIcon} from 'lucide-react';
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
    <article className="group rounded-xl bg-canvas-soft p-5 transition-colors duration-120 hover:bg-control-surface">
      <div className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-2">
          {hideTopicLink ? (
            <Badge variant="secondary">{topicMeta.label}</Badge>
          ) : (
            <Badge variant="secondary" asChild>
              <Link href={`/topics/${topicMeta.slug}`}>{topicMeta.label}</Link>
            </Badge>
          )}
          {post.scope === 'course' ? (
            <Badge variant="secondary">课程讨论</Badge>
          ) : null}
          <span className="text-xs leading-5 text-muted-foreground">
            {authorLabel}
          </span>
          <span
            aria-hidden="true"
            className="text-xs leading-5 text-muted-foreground"
          >
            ·
          </span>
          <time
            dateTime={post.createdAt}
            className="text-xs leading-5 text-muted-foreground"
          >
            {formatDateTime(post.createdAt)}
          </time>
        </div>

        <div className="flex flex-col gap-2">
          <h3 className="text-base leading-[26px] font-medium text-foreground">
            <Link
              href={`/community/${post.id}`}
              className="line-clamp-2 rounded-xs underline-offset-4 outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
            >
              {post.title}
            </Link>
          </h3>

          <p className="line-clamp-3 text-sm leading-6 text-body">
            {post.publicSummary?.trim() || '作者未提供补充摘要。'}
          </p>
        </div>

        <div className="flex items-center justify-between pt-1">
          <Link
            href={`/community/${post.id}`}
            className="inline-flex items-center gap-1 rounded-xs text-[13px] leading-5 font-medium text-foreground underline decoration-foreground/40 underline-offset-4 transition-colors outline-none hover:decoration-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            查看讨论
            <ArrowUpRightIcon className="size-3.5" aria-hidden="true" />
          </Link>
        </div>
      </div>
    </article>
  );
}
