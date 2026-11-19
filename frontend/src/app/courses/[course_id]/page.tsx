import type {Metadata} from 'next';
import Link from 'next/link';
import {notFound} from 'next/navigation';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {getServerRequestOptions} from '@/lib/request/server';
import {learningGetCourse} from '@/services/generated/learning/courses';

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
      title: `${course.title} · ai2workspace 课程`,
      description: course.description || '课程学习大纲与课时内容。',
    };
  } catch {
    return {
      title: '课程详情 · ai2workspace',
    };
  }
}

export const dynamic = 'force-dynamic';

export default async function CourseDetailPage({
  params,
}: CourseDetailPageProps) {
  const {course_id} = await params;

  let course: LearningApi.CourseResponse | null = null;
  try {
    const options = await getServerRequestOptions();
    course = await learningGetCourse({id: course_id}, options);
  } catch {
    notFound();
  }

  if (!course) {
    notFound();
  }

  return (
    <main className="mx-auto flex min-h-svh w-full max-w-4xl flex-col px-4 py-12 sm:px-6 space-y-8">
      {/* 顶部课程信息 */}
      <div className="space-y-4 border-b pb-6">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={course.isFree ? 'secondary' : 'default'}>
            {course.isFree ? '免费课' : '付费课程'}
          </Badge>
          <Badge variant="outline">版本 v{course.publishedVersion}</Badge>
          <Badge
            variant={
              course.deliveryState === 'available' ? 'outline' : 'destructive'
            }
          >
            {course.deliveryState === 'available' ? '正常交付' : '已下架'}
          </Badge>
        </div>

        <h1 className="text-3xl font-semibold tracking-tight">
          {course.title}
        </h1>
        <p className="text-muted-foreground text-sm sm:text-base leading-relaxed">
          {course.description || '暂无详细介绍'}
        </p>

        <div className="pt-2 flex items-center gap-3">
          <Button asChild size="default">
            <Link href={`/account`}>登录并加入学习</Link>
          </Button>
          <Button asChild variant="outline">
            <Link href="/courses">返回课程广场</Link>
          </Button>
        </div>
      </div>

      {/* 章节大纲 */}
      <div className="space-y-4">
        <h2 className="text-xl font-semibold tracking-tight">课程大纲</h2>

        {course.sections && course.sections.length > 0 ? (
          <div className="space-y-4">
            {course.sections.map((section, idx) => (
              <Card key={section.id} className="shadow-sm">
                <CardHeader className="py-3 px-4 bg-muted/30">
                  <CardTitle className="text-sm font-medium">
                    第 {idx + 1} 章 · {section.title}
                  </CardTitle>
                </CardHeader>
                <CardContent className="divide-y p-0">
                  {section.lessons && section.lessons.length > 0 ? (
                    section.lessons.map((lesson) => (
                      <div
                        key={lesson.id}
                        className="flex items-center justify-between p-3.5 text-xs hover:bg-muted/10 transition-colors"
                      >
                        <div className="flex items-center gap-2">
                          <span className="font-mono text-muted-foreground">
                            {lesson.type === 'video' ? '🎬' : '📄'}
                          </span>
                          <span className="font-medium">{lesson.title}</span>
                          {lesson.isTrial && (
                            <Badge
                              variant="secondary"
                              className="text-[10px] py-0 px-1.5"
                            >
                              试看
                            </Badge>
                          )}
                        </div>
                        <span className="text-[10px] text-muted-foreground uppercase">
                          {lesson.type}
                        </span>
                      </div>
                    ))
                  ) : (
                    <div className="p-3 text-xs text-muted-foreground italic">
                      本章暂无课时
                    </div>
                  )}
                </CardContent>
              </Card>
            ))}
          </div>
        ) : (
          <Card className="p-6 text-center text-xs text-muted-foreground">
            讲师正在积极组织章节大纲，敬请期待。
          </Card>
        )}
      </div>
    </main>
  );
}
