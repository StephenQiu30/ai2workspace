'use client';

import {useState} from 'react';
import Link from 'next/link';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {Input} from '@/components/ui/input';

export interface CatalogCourse {
  id: string;
  title: string;
  description?: string;
  coverUrl?: string;
  isFree?: boolean;
  publishedVersion?: number;
  sectionsCount?: number;
  lessonsCount?: number;
}

interface CourseCatalogProps {
  readonly initialCourses?: CatalogCourse[];
}

export function CourseCatalog({initialCourses = []}: CourseCatalogProps) {
  const [query, setQuery] = useState('');

  const filtered = initialCourses.filter((c) =>
    c.title.toLowerCase().includes(query.toLowerCase().trim()),
  );

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <Input
          type="search"
          placeholder="搜索课程标题..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="max-w-md h-10"
        />
      </div>

      {filtered.length === 0 ? (
        <Card className="text-center py-12 border-dashed">
          <CardContent className="space-y-2">
            <p className="text-sm font-medium text-muted-foreground">
              暂无可用的公开课程
            </p>
            <p className="text-xs text-muted-foreground">
              讲师发布并通过审核的课程将在此展示
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filtered.map((course) => (
            <Card
              key={course.id}
              className="flex flex-col justify-between hover:border-primary/50 transition-colors"
            >
              <CardHeader className="space-y-2">
                <div className="flex items-center justify-between gap-2">
                  <Badge
                    variant={course.isFree ? 'secondary' : 'default'}
                    className="text-xs"
                  >
                    {course.isFree ? '免费课' : '精品付费'}
                  </Badge>
                  {course.publishedVersion && (
                    <span className="text-[10px] text-muted-foreground">
                      v{course.publishedVersion}
                    </span>
                  )}
                </div>
                <CardTitle className="text-lg line-clamp-2">
                  {course.title}
                </CardTitle>
                <CardDescription className="text-xs line-clamp-3">
                  {course.description || '暂无课程描述'}
                </CardDescription>
              </CardHeader>
              <CardFooter className="border-t pt-4">
                <Button
                  asChild
                  variant="outline"
                  className="w-full h-9 text-xs"
                >
                  <Link href={`/courses/${course.id}`}>进入学习详情</Link>
                </Button>
              </CardFooter>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
