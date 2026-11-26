import {Layout} from '@/components/layout/layout';
import type {Metadata} from 'next';
import Link from 'next/link';
import {notFound} from 'next/navigation';
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb';
import {
  CourseDetailClient,
  type CourseAccessSource,
} from '@/features/course/course_detail_client';
import {isAdmin} from '@/lib/auth/roles';
import {getServerSession} from '@/lib/auth/session';
import {getServerRequestOptions} from '@/lib/request/server';
import {
  commerceGetCoursePrice,
  commerceListMyEntitlements,
} from '@/services/generated/api/commerce';
import {
  learningGetCourse,
  learningListMyCourses,
} from '@/services/generated/api/courses';

interface CourseDetailPageProps {
  readonly params: Promise<{course_id: string}>;
}

export async function generateMetadata({
  params,
}: CourseDetailPageProps): Promise<Metadata> {
  const {course_id} = await params;
  try {
    const options = await getServerRequestOptions();
    const course = await learningGetCourse({id: course_id}, options);
    return {
      title: course.title || '课程详情',
      description:
        course.description || '查看学屿课程详情、章节大纲与试看内容。',
    };
  } catch {
    return {
      title: '课程详情',
    };
  }
}

export const dynamic = 'force-dynamic';

export default async function CourseDetailPage({
  params,
}: CourseDetailPageProps) {
  const {course_id: courseId} = await params;
  const options = await getServerRequestOptions();

  const [courseResult, priceResult, user] = await Promise.all([
    learningGetCourse({id: courseId}, options).catch(() => null),
    commerceGetCoursePrice({id: courseId}, options).catch(() => null),
    getServerSession(),
  ]);

  if (!courseResult || !courseResult.id) {
    notFound();
  }

  let hasAccess = false;
  let accessSource: CourseAccessSource = null;
  let lastLessonId: string | null = null;

  if (user) {
    const isStaffOrOwner =
      isAdmin(user) ||
      (Boolean(courseResult.ownerId) && courseResult.ownerId === user.id);

    const [myCourses, myEntitlements] = await Promise.all([
      learningListMyCourses(options).catch(() => []),
      commerceListMyEntitlements(options).catch(() => []),
    ]);

    const matchedMyCourse = (Array.isArray(myCourses) ? myCourses : []).find(
      (item) => item.id === courseResult.id,
    );
    const hasActiveEntitlement = (
      Array.isArray(myEntitlements) ? myEntitlements : []
    ).some(
      (item) =>
        item.courseId === courseResult.id &&
        (!item.status || item.status === 'active'),
    );

    if (matchedMyCourse?.progressSummary?.lastLessonId) {
      lastLessonId = matchedMyCourse.progressSummary.lastLessonId;
    }

    if (isStaffOrOwner) {
      hasAccess = true;
      accessSource = 'staff';
    } else if (
      hasActiveEntitlement ||
      matchedMyCourse?.enrollmentType === 'purchased'
    ) {
      hasAccess = true;
      accessSource = 'purchased';
    } else if (matchedMyCourse) {
      hasAccess = true;
      accessSource = courseResult.isFree ? 'free' : 'purchased';
    }
  }

  return (
    <Layout flow="stack" className="w-full gap-6">
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link href="/courses">课程目录</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>{courseResult.title || '课程详情'}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <CourseDetailClient
        course={courseResult}
        price={priceResult}
        user={user}
        hasAccess={hasAccess}
        accessSource={accessSource}
        lastLessonId={lastLessonId}
      />
    </Layout>
  );
}
