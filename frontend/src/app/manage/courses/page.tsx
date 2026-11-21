import type {Metadata} from 'next';
import {ManageNav} from '@/components/layout/manage_nav';
import {ManageAccessGuard} from '@/features/assistant/manage_access_guard';
import {CourseManageList} from '@/features/course/course_manage_list';
import {canManageCourses} from '@/lib/auth/roles';
import {getServerSession} from '@/lib/auth/session';
import {getServerRequestOptions} from '@/lib/request/server';
import {learningListManageCourses} from '@/services/generated/learning/courses';

export const metadata: Metadata = {
  title: '课程管理',
  description:
    '讲师与管理员课程供给工作台：新建草稿、编辑大纲、提交审核与上下架治理。',
};

export const dynamic = 'force-dynamic';

export default async function ManageCoursesPage() {
  const user = await getServerSession();

  if (!canManageCourses(user) || !user) {
    return (
      <ManageAccessGuard
        user={user}
        currentPath="/manage/courses"
        requiredRoleLabel="讲师或管理员"
      />
    );
  }

  let courses: LearningApi.CourseResponse[] = [];
  let loadFailed = false;

  try {
    const options = await getServerRequestOptions();
    const resp = await learningListManageCourses(
      {page: 1, pageSize: 100},
      options,
    );
    courses = Array.isArray(resp?.courses) ? resp.courses : [];
  } catch {
    loadFailed = true;
  }

  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col">
      <ManageNav user={user} />

      <div className="mb-8 flex flex-col gap-2">
        <h1 className="text-[28px] leading-[40px] font-medium text-foreground">
          课程管理
        </h1>
        <p className="text-sm leading-6 text-body">
          集中管理课程草稿、章节课时编排、上线审核与售卖交付状态。
        </p>
      </div>

      <CourseManageList
        initialCourses={courses}
        user={user}
        loadFailed={loadFailed}
      />
    </div>
  );
}
