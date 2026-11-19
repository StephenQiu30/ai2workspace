import type {Metadata} from 'next';
import {
  CourseCatalog,
  type CatalogCourse,
} from '@/features/course/course_catalog';
import {getServerRequestOptions} from '@/lib/request/server';
import {learningListCourses} from '@/services/generated/learning/courses';

export const metadata: Metadata = {
  title: '课程广场 · ai2workspace',
  description: '浏览已发布的全栈工程、AI 与系统开发实战课程。',
};

export const dynamic = 'force-dynamic';

export default async function CoursesPage() {
  let initialCourses: CatalogCourse[] = [];

  try {
    const options = await getServerRequestOptions();
    const resp = await learningListCourses({page: 1, limit: 30}, options);
    if (resp?.courses) {
      initialCourses = resp.courses
        .filter((c): c is typeof c & {id: string; title: string} =>
          Boolean(c.id && c.title),
        )
        .map((c) => ({
          id: c.id,
          title: c.title,
          description: c.description,
          coverUrl: c.coverUrl,
          isFree: c.isFree,
          publishedVersion: c.publishedVersion,
          sectionsCount: c.sections?.length ?? 0,
          lessonsCount:
            c.sections?.reduce((acc, s) => acc + (s.lessons?.length ?? 0), 0) ??
            0,
        }));
    }
  } catch {
    // Graceful fallback when service is temporarily unavailable
  }

  return (
    <main className="mx-auto flex min-h-svh w-full max-w-6xl flex-col px-4 py-12 sm:px-6">
      <div className="mb-8">
        <h1 className="text-3xl font-semibold tracking-tight">课程广场</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          精选高品质实战课程，支持试读试看与阶段性学习进度记录
        </p>
      </div>
      <CourseCatalog initialCourses={initialCourses} />
    </main>
  );
}
