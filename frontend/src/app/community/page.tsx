import Link from 'next/link';
import {communityListPosts} from '@/services/generated/community/community';
import {PostCard} from '@/features/community/post_card';
import {PostEditor} from '@/features/community/post_editor';
import {Button} from '@/components/ui/button';

interface CommunityPageProps {
  searchParams: Promise<{
    scope?: string;
    course_id?: string;
  }>;
}

export default async function CommunityPage({
  searchParams,
}: CommunityPageProps) {
  const {scope = 'global', course_id: courseId} = await searchParams;

  let postsList: CommunityApi.PostResponse[] = [];
  try {
    const resp = await communityListPosts({
      scope,
      courseId,
      page: 1,
      limit: 50,
    });
    postsList = resp.items || [];
  } catch {
    postsList = [];
  }

  return (
    <div className="container mx-auto px-4 py-8 max-w-5xl space-y-8">
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">学习讨论社区</h1>
          <p className="text-xs text-muted-foreground mt-1">
            分享技术心得、探讨工程设计、交流课程疑问
          </p>
        </div>

        <div className="flex items-center space-x-2 text-xs">
          <Link href="/community?scope=global">
            <Button
              variant={scope === 'global' ? 'default' : 'outline'}
              size="sm"
            >
              全站讨论
            </Button>
          </Link>
          <Link href="/community?scope=course">
            <Button
              variant={scope === 'course' ? 'default' : 'outline'}
              size="sm"
            >
              课程专区
            </Button>
          </Link>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Posts feed */}
        <div className="lg:col-span-2 space-y-4">
          {postsList.length === 0 ? (
            <div className="p-12 text-center border rounded-lg bg-muted/20 text-muted-foreground text-xs">
              该板块暂无讨论话题，欢迎发布第一个话题！
            </div>
          ) : (
            postsList.map((p) => <PostCard key={p.id} post={p} />)
          )}
        </div>

        {/* Create post sidebar */}
        <div>
          <PostEditor
            initialScope={scope as 'global' | 'course'}
            courseId={courseId}
          />
        </div>
      </div>
    </div>
  );
}
