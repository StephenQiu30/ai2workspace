import type {Metadata} from 'next';
import {loadHomePageData} from '@/features/community/community_data';
import {CommunityHomeView} from '@/features/community/community_home_view';
import {getServerSession} from '@/lib/auth/session';

export const metadata: Metadata = {
  title: '创作社区',
  description: '学习 AI 短漫剧制作和其他创作课程，参与作品讨论。',
};

export const dynamic = 'force-dynamic';

interface HomePageProps {
  searchParams: Promise<{
    topic?: string | string[];
    compose?: string;
    course?: string;
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
      key={JSON.stringify([resolvedParams.compose, resolvedParams.course])}
      composeTopic={
        ['general', 'services', 'course-qa'].includes(
          resolvedParams.compose ?? '',
        )
          ? resolvedParams.compose
          : undefined
      }
      courseTitle={
        typeof resolvedParams.course === 'string'
          ? resolvedParams.course.slice(0, 90)
          : undefined
      }
      basePath="/"
      user={user}
      activeTopic={data.activeTopic}
      page={data.page}
      pageSize={data.pageSize}
      courses={data.courses}
      coursesError={data.coursesError}
      services={data.services}
      servicesError={data.servicesError}
      posts={data.posts}
      postsTotal={data.postsTotal}
      postsError={data.postsError}
      topics={data.topics}
      myCourses={data.myCourses}
      myCoursesError={data.myCoursesError}
    />
  );
}
