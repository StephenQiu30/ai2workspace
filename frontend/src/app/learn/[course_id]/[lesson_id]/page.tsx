import type {Metadata} from 'next';
import {notFound} from 'next/navigation';
import {LessonPlayer} from '@/features/learning/lesson_player';
import {getServerSession} from '@/lib/auth/session';
import {getServerRequestOptions} from '@/lib/request/server';
import {communityListPosts} from '@/services/generated/api/community';
import {learningGetCourse} from '@/services/generated/api/courses';

interface LearnPageProps {
  readonly params: Promise<{
    course_id: string;
    lesson_id: string;
  }>;
}

export async function generateMetadata({
  params,
}: LearnPageProps): Promise<Metadata> {
  const {course_id: courseId, lesson_id: lessonId} = await params;
  try {
    const options = await getServerRequestOptions();
    const course = await learningGetCourse({id: courseId}, options);
    let lessonTitle = '';
    for (const sec of course.sections ?? []) {
      const found = (sec.lessons ?? []).find((item) => item.id === lessonId);
      if (found?.title) {
        lessonTitle = found.title;
        break;
      }
    }
    return {
      title: lessonTitle
        ? `${lessonTitle} · ${course.title ?? '课时学习'}`
        : course.title || '课时学习',
    };
  } catch {
    return {
      title: '课时学习',
    };
  }
}

export const dynamic = 'force-dynamic';

export default async function LearnPage({params}: LearnPageProps) {
  const {course_id: courseId, lesson_id: lessonId} = await params;
  const options = await getServerRequestOptions();

  const [course, postsResp, user] = await Promise.all([
    learningGetCourse({id: courseId}, options).catch(() => null),
    communityListPosts(
      {scope: 'course', courseId, page: 1, pageSize: 10},
      options,
    ).catch(() => null),
    getServerSession(),
  ]);

  if (!course || !course.id) {
    notFound();
  }

  let activeLesson: Api.Lesson | undefined;
  for (const section of course.sections ?? []) {
    const found = (section.lessons ?? []).find((item) => item.id === lessonId);
    if (found) {
      activeLesson = found;
      break;
    }
  }

  if (!activeLesson || !activeLesson.id) {
    notFound();
  }

  const discussions = postsResp?.items ?? [];

  return (
    <LessonPlayer
      courseId={course.id}
      lessonId={activeLesson.id}
      isTrial={Boolean(activeLesson.isTrial)}
      contentVersion={course.publishedVersion ?? course.version ?? 1}
      lessonTitle={activeLesson.title || '课时学习'}
      lessonType={activeLesson.type || 'video'}
      lessonContent={activeLesson.content}
      course={course}
      discussions={discussions}
      user={user}
    />
  );
}
