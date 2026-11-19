import type {Metadata} from 'next';
import {CourseEditor} from '@/features/course/course_editor';

export const metadata: Metadata = {
  title: '课程与大纲管理 · ai2workspace',
  description:
    '讲师与管理员课程制作控制台：创建草稿、章节大纲、审核与发布治理。',
};

export const dynamic = 'force-dynamic';

export default function ManageCoursesPage() {
  return (
    <main className="mx-auto flex min-h-svh w-full max-w-4xl flex-col px-4 py-12 sm:px-6">
      <div className="mb-8">
        <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">
          课程供给与治理工作台
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          受控创建课程草稿、组织章节课时大纲、提交管理员审核及上下架处置
        </p>
      </div>
      <CourseEditor />
    </main>
  );
}
