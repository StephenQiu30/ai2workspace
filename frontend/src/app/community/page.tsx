import type {Metadata} from 'next';
import {loadHomePageData} from '@/features/community/community_data';
import {CommunityHomeView} from '@/features/community/community_home_view';
import {getServerSession} from '@/lib/auth/session';

export const metadata: Metadata = {
  title: '讨论社区',
  description: '浏览全部公开讨论话题、独立摘要与精选课程。',
};

export const dynamic = 'force-dynamic';

interface CommunityPageProps {
  searchParams: Promise<{
    topic?: string | string[];
    page?: string | string[];
  }>;
}

export default async function CommunityPage({
  searchParams,
}: CommunityPageProps) {
  const [resolvedParams, user] = await Promise.all([
    searchParams,
    getServerSession(),
  ]);
  const data = await loadHomePageData({
    topic: resolvedParams.topic,
    page: resolvedParams.page,
    user,
  });

  return (
    <CommunityHomeView
      basePath="/community"
      user={user}
      activeTopic={data.activeTopic}
      page={data.page}
      pageSize={data.pageSize}
      courses={data.courses}
      coursesError={data.coursesError}
      posts={data.posts}
      postsTotal={data.postsTotal}
      postsError={data.postsError}
      topics={data.topics}
      myCourses={data.myCourses}
      myCoursesError={data.myCoursesError}
    />
  );
}
