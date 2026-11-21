import type {Metadata} from 'next';
import {loadHomePageData} from '@/features/community/community_data';
import {CommunityHomeView} from '@/features/community/community_home_view';
import {getServerSession} from '@/lib/auth/session';

export const metadata: Metadata = {
  title: '学屿社区',
  description: '先发现结构化课程，再围绕真实学习问题与实践经验持续交流。',
};

export const dynamic = 'force-dynamic';

interface HomePageProps {
  searchParams: Promise<{
    topic?: string | string[];
    page?: string | string[];
  }>;
}

export default async function HomePage({searchParams}: HomePageProps) {
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
      basePath="/"
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
