import Link from 'next/link';
import {notFound} from 'next/navigation';
import {communityGetPost} from '@/services/generated/community/community';
import {PostDetailView} from '@/features/community/post_detail_view';
import {Button} from '@/components/ui/button';

interface PostDetailPageProps {
  params: Promise<{
    post_id: string;
  }>;
}

export default async function PostDetailPage({params}: PostDetailPageProps) {
  const {post_id: postId} = await params;

  let post: CommunityApi.PostDetailResponse | null = null;
  try {
    post = await communityGetPost({id: postId});
  } catch {
    notFound();
  }

  if (!post) {
    notFound();
  }

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl space-y-6">
      <div className="flex justify-between items-center">
        <Link href="/community">
          <Button variant="ghost" size="sm" className="text-xs">
            ← 返回社区列表
          </Button>
        </Link>
      </div>

      <PostDetailView post={post} />
    </div>
  );
}
