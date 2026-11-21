import type {Metadata} from 'next';
import {notFound} from 'next/navigation';
import {loadTopicPageData} from '@/features/community/community_data';
import {TopicDetailView} from '@/features/community/topic_detail_view';
import {getServerSession} from '@/lib/auth/session';
import {getTopicMeta} from '@/lib/format';

export const dynamic = 'force-dynamic';

interface TopicPageProps {
  params: Promise<{
    slug: string;
  }>;
  searchParams: Promise<{
    page?: string | string[];
  }>;
}

export async function generateMetadata({
  params,
}: TopicPageProps): Promise<Metadata> {
  const {slug} = await params;
  const meta = getTopicMeta(slug);
  return {
    title: `${meta.label} · 话题讨论`,
    description: meta.description,
  };
}

export default async function TopicDetailPage({
  params,
  searchParams,
}: TopicPageProps) {
  const [{slug}, resolvedSearchParams, user] = await Promise.all([
    params,
    searchParams,
    getServerSession(),
  ]);

  const data = await loadTopicPageData({
    slug,
    page: resolvedSearchParams.page,
  });

  if (!data.topic) {
    notFound();
  }

  return (
    <TopicDetailView
      user={user}
      topic={data.topic}
      topics={data.topics}
      posts={data.posts}
      postsTotal={data.postsTotal}
      page={data.page}
      pageSize={data.pageSize}
      postsError={data.postsError}
    />
  );
}
