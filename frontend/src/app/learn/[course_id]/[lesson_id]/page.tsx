import Link from 'next/link';
import {notFound} from 'next/navigation';
import {learningGetCourse} from '@/services/generated/learning/courses';
import {LessonPlayer} from '@/features/learning/lesson_player';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';

interface LearnPageProps {
  params: Promise<{
    course_id: string;
    lesson_id: string;
  }>;
}

export default async function LearnPage({params}: LearnPageProps) {
  const {course_id: courseId, lesson_id: lessonId} = await params;

  let course: LearningApi.CourseResponse | null = null;
  try {
    course = await learningGetCourse({id: courseId});
  } catch {
    notFound();
  }

  if (!course) {
    notFound();
  }

  // Find active lesson
  let activeLesson: LearningApi.Lesson | undefined;
  for (const section of course.sections || []) {
    const found = (section.lessons || []).find((l) => l.id === lessonId);
    if (found) {
      activeLesson = found;
      break;
    }
  }

  if (!activeLesson) {
    notFound();
  }

  return (
    <div className="container mx-auto px-4 py-6 max-w-6xl space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <nav className="text-xs text-muted-foreground mb-1 flex items-center space-x-1">
            <Link href="/courses" className="hover:underline">
              课程广场
            </Link>
            <span>/</span>
            <Link href={`/courses/${course.id}`} className="hover:underline">
              {course.title}
            </Link>
            <span>/</span>
            <span className="text-foreground">{activeLesson.title}</span>
          </nav>
          <h1 className="text-xl font-bold tracking-tight">{course.title}</h1>
        </div>

        <Link href={`/courses/${course.id}`}>
          <Button variant="outline" size="sm">
            返回课程大纲
          </Button>
        </Link>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Main Lesson Player */}
        <div className="lg:col-span-2 space-y-4">
          <LessonPlayer
            courseId={course.id || ''}
            lessonId={activeLesson.id || ''}
            isTrial={Boolean(activeLesson.isTrial)}
            contentVersion={course.version || 1}
            lessonTitle={activeLesson.title || ''}
          />

          {activeLesson.content && (
            <Card>
              <CardHeader className="py-3 px-4">
                <CardTitle className="text-sm font-semibold">
                  课时配套图文资料
                </CardTitle>
              </CardHeader>
              <CardContent className="text-xs leading-relaxed text-muted-foreground whitespace-pre-wrap">
                {activeLesson.content}
              </CardContent>
            </Card>
          )}
        </div>

        {/* Section & Lesson Outline */}
        <div className="space-y-4">
          <Card>
            <CardHeader className="py-3 px-4">
              <CardTitle className="text-sm font-semibold">
                课程目录与进度
              </CardTitle>
            </CardHeader>
            <CardContent className="p-2 space-y-3">
              {(course.sections || []).map((sec, sIdx) => (
                <div key={sec.id || sIdx} className="space-y-1">
                  <div className="text-xs font-medium px-2 py-1 text-muted-foreground bg-muted/30 rounded">
                    第 {sIdx + 1} 章：{sec.title}
                  </div>
                  <div className="space-y-0.5">
                    {(sec.lessons || []).map((lsn) => {
                      const isCurrent = lsn.id === activeLesson.id;
                      return (
                        <Link
                          key={lsn.id}
                          href={`/learn/${course.id}/${lsn.id}`}
                          className={`flex items-center justify-between px-3 py-2 text-xs rounded transition-colors ${
                            isCurrent
                              ? 'bg-primary text-primary-foreground font-medium'
                              : 'hover:bg-muted text-muted-foreground hover:text-foreground'
                          }`}
                        >
                          <div className="flex items-center space-x-2 truncate">
                            <span>{lsn.type === 'video' ? '▶' : '📄'}</span>
                            <span className="truncate">{lsn.title}</span>
                          </div>
                          {lsn.isTrial && (
                            <span
                              className={`text-[10px] px-1.5 py-0.5 rounded font-normal ${
                                isCurrent
                                  ? 'bg-primary-foreground/20 text-primary-foreground'
                                  : 'bg-primary/10 text-primary'
                              }`}
                            >
                              试看
                            </span>
                          )}
                        </Link>
                      );
                    })}
                  </div>
                </div>
              ))}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
