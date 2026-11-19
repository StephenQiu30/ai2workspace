import Link from 'next/link';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Badge} from '@/components/ui/badge';

export interface PostCardProps {
  post: CommunityApi.PostResponse;
}

export function PostCard({post}: PostCardProps) {
  return (
    <Card className="hover:border-primary/50 transition-colors">
      <CardHeader className="py-3 px-4 pb-2">
        <div className="flex justify-between items-start gap-2">
          <Link
            href={`/community/${post.id}`}
            className="hover:underline flex-1"
          >
            <CardTitle className="text-base font-semibold leading-snug">
              {post.title}
            </CardTitle>
          </Link>
          <Badge
            variant={post.scope === 'course' ? 'secondary' : 'outline'}
            className="text-[10px]"
          >
            {post.scope === 'course' ? '课程讨论' : '全站社区'}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="px-4 pb-3 space-y-2 text-xs">
        <p className="text-muted-foreground line-clamp-2 leading-relaxed">
          {post.publicSummary || '（暂无公开摘要）'}
        </p>
        <div className="flex items-center justify-between text-muted-foreground pt-1 border-t text-[11px]">
          <div className="flex items-center space-x-2">
            <span>
              作者: <span className="font-mono">{post.authorId}</span>
            </span>
          </div>
          <div>
            {post.createdAt
              ? new Date(post.createdAt).toLocaleDateString()
              : ''}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
