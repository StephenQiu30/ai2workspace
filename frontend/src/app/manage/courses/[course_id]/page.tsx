import {Layout} from '@/components/layout/layout';
import {Heading, Text} from '@/components/layout/typography';
import type {Metadata} from 'next';
import Link from 'next/link';
import {notFound} from 'next/navigation';
import {ManageNav} from '@/components/layout/manage_nav';
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb';
import {ManageAccessGuard} from '@/features/assistant/manage_access_guard';
import {CourseEditor} from '@/features/course/course_editor';
import {canManageCourses} from '@/lib/auth/roles';
import {getServerSession} from '@/lib/auth/session';
import {getServerRequestOptions} from '@/lib/request/server';
import {commerceGetCoursePrice} from '@/services/generated/api/commerce';
import {learningGetCourse} from '@/services/generated/api/courses';

interface ManageCourseEditPageProps {
  readonly params: Promise<{course_id: string}>;
}

export async function generateMetadata({
  params,
}: ManageCourseEditPageProps): Promise<Metadata> {
  const {course_id: courseId} = await params;
  try {
    const options = await getServerRequestOptions();
    const course = await learningGetCourse({id: courseId}, options);
    return {
      title: `${course.title || '编辑课程'} · 课程管理`,
      description: '编辑课程基础信息、人民币定价、章节课时大纲与审核发布状态。',
    };
  } catch {
    return {
      title: '编辑课程 · 课程管理',
    };
  }
}

export const dynamic = 'force-dynamic';

export default async function ManageCourseEditPage({
  params,
}: ManageCourseEditPageProps) {
  const {course_id: courseId} = await params;
  const user = await getServerSession();

  if (!canManageCourses(user) || !user) {
    return (
      <ManageAccessGuard
        user={user}
        currentPath={`/manage/courses/${courseId}`}
        requiredRoleLabel="讲师或管理员"
      />
    );
  }

  const options = await getServerRequestOptions();
  const [course, price] = await Promise.all([
    learningGetCourse({id: courseId}, options).catch(() => null),
    commerceGetCoursePrice({id: courseId}, options).catch(() => null),
  ]);

  if (!course || !course.id) {
    notFound();
  }

  return (
    <Layout flow="stack" className="w-full">
      <ManageNav user={user} />

      <Layout flow="stack" className="mb-6 gap-2">
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbLink asChild>
                <Link href="/manage/courses">课程管理</Link>
              </BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <BreadcrumbPage>{course.title || '课程编辑'}</BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>

        <Heading level={1} size="page" className="text-foreground">
          课程编辑与审核发布
        </Heading>
        <Text as="p" variant="body">
          维护课程元信息、人民币买断定价、章节与视频/图文课时，并执行提审或管理员上线处置。
        </Text>
      </Layout>

      <CourseEditor initialCourse={course} initialPrice={price} user={user} />
    </Layout>
  );
}
