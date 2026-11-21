import type {Metadata} from 'next';
import {
  CourseCatalog,
  type CatalogCourse,
} from '@/features/course/course_catalog';
import {countCourseLessons} from '@/features/course/course_status';
import {getServerRequestOptions} from '@/lib/request/server';
import {commerceGetCoursePrice} from '@/services/generated/commerce/commerce';
import {learningListCourses} from '@/services/generated/learning/courses';

export const metadata: Metadata = {
  title: '课程目录',
  description:
    '按标题搜索或分类筛选学屿已发布课程，支持免费公开课与单课买断学习。',
};

export const dynamic = 'force-dynamic';

const PAGE_SIZE = 12;

interface CoursesPageProps {
  readonly searchParams: Promise<{
    q?: string;
    category?: string;
    page?: string;
  }>;
}

export default async function CoursesPage({searchParams}: CoursesPageProps) {
  const resolved = await searchParams;
  const query = typeof resolved.q === 'string' ? resolved.q.trim() : '';
  const category =
    typeof resolved.category === 'string' && resolved.category.trim()
      ? resolved.category.trim()
      : 'all';
  const parsedPage = Number.parseInt(resolved.page ?? '1', 10);
  const page = Number.isFinite(parsedPage) && parsedPage > 0 ? parsedPage : 1;

  let initialCourses: CatalogCourse[] = [];
  let total = 0;
  let loadFailed = false;

  try {
    const options = await getServerRequestOptions();
    const resp = await learningListCourses(
      {
        q: query || undefined,
        category: category !== 'all' ? category : undefined,
        page,
        pageSize: PAGE_SIZE,
      },
      options,
    );

    const rawCourses = (resp?.courses ?? []).filter(
      (
        course,
      ): course is LearningApi.CourseResponse & {id: string; title: string} =>
        Boolean(course.id && course.title),
    );
    total = resp?.total ?? rawCourses.length;

    const priceEntries = await Promise.all(
      rawCourses.map(async (course) => {
        if (course.isFree) {
          return [course.id, 0] as const;
        }
        try {
          const price = await commerceGetCoursePrice({id: course.id}, options);
          return [course.id, price?.amountMinor] as const;
        } catch {
          return [course.id, undefined] as const;
        }
      }),
    );
    const priceMap = new Map<string, number | undefined>(priceEntries);

    initialCourses = rawCourses.map((course) => ({
      id: course.id,
      title: course.title,
      description: course.description,
      coverUrl: course.coverUrl,
      category: course.category,
      authorNickname: course.authorNickname,
      isFree: course.isFree,
      priceAmountMinor: priceMap.get(course.id),
      publishedVersion: course.publishedVersion,
      sectionsCount: course.sections?.length ?? 0,
      lessonsCount: countCourseLessons(course),
      saleState: course.saleState,
      deliveryState: course.deliveryState,
    }));
  } catch {
    loadFailed = true;
  }

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col gap-8">
      <section className="flex flex-col gap-2">
        <h1 className="text-[28px] leading-[40px] font-medium text-foreground">
          课程目录
        </h1>
        <p className="text-sm leading-6 text-body">
          浏览已通过审核的公开课程，支持免费试看、单课买断与跨设备学习进度记录。
        </p>
      </section>

      <CourseCatalog
        initialCourses={initialCourses}
        total={total}
        page={page}
        pageSize={PAGE_SIZE}
        initialQuery={query}
        initialCategory={category}
        loadFailed={loadFailed}
      />
    </div>
  );
}
