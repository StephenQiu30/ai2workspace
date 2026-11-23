'use client';

import {Layout} from '@/components/layout/layout';
import {Text} from '@/components/layout/typography';
import {useState} from 'react';
import Link from 'next/link';
import {CourseCover} from './course_cover';
import {useRouter} from 'next/navigation';
import {
  AlertCircleIcon,
  ArrowRightIcon,
  BookOpenIcon,
  SearchIcon,
  UserIcon,
  XIcon,
} from 'lucide-react';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
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
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from '@/components/ui/input-group';
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  COURSE_CATEGORIES,
  formatPriceCny,
  getCourseCategoryLabel,
} from '@/lib/format';

export interface CatalogCourse {
  id: string;
  title: string;
  description?: string;
  coverUrl?: string;
  category?: string;
  authorNickname?: string;
  isFree?: boolean;
  priceAmountMinor?: number;
  publishedVersion?: number;
  sectionsCount?: number;
  lessonsCount?: number;
  saleState?: string;
  deliveryState?: string;
}

export interface CourseCatalogProps {
  readonly initialCourses?: CatalogCourse[];
  readonly total?: number;
  readonly page?: number;
  readonly pageSize?: number;
  readonly initialQuery?: string;
  readonly initialCategory?: string;
  readonly loadFailed?: boolean;
}

function buildCoursesHref(
  query: string,
  category: string,
  page: number,
): string {
  const params = new URLSearchParams();
  const trimmed = query.trim();
  if (trimmed) {
    params.set('q', trimmed);
  }
  if (category && category !== 'all') {
    params.set('category', category);
  }
  if (page > 1) {
    params.set('page', String(page));
  }
  const serialized = params.toString();
  return serialized ? `/courses?${serialized}` : '/courses';
}

export function CourseCatalog({
  initialCourses = [],
  total = initialCourses.length,
  page = 1,
  pageSize = 12,
  initialQuery = '',
  initialCategory = 'all',
  loadFailed = false,
}: CourseCatalogProps) {
  const router = useRouter();
  const [queryInput, setQueryInput] = useState(initialQuery);

  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const hasActiveFilter =
    Boolean(initialQuery.trim()) ||
    (Boolean(initialCategory) && initialCategory !== 'all');

  function applyFilters(nextQuery: string, nextCategory: string) {
    router.push(buildCoursesHref(nextQuery, nextCategory, 1));
  }

  function handleSearchSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    applyFilters(queryInput, initialCategory);
  }

  function handleClearSearch() {
    setQueryInput('');
    applyFilters('', initialCategory);
  }

  function handleCategoryChange(nextCategory: string) {
    applyFilters(queryInput, nextCategory);
  }

  function handleResetAll() {
    setQueryInput('');
    router.push('/courses');
  }

  const pageNumbers: number[] = [];
  const startPage = Math.max(1, page - 2);
  const endPage = Math.min(totalPages, startPage + 4);
  for (let current = startPage; current <= endPage; current += 1) {
    pageNumbers.push(current);
  }

  return (
    <Layout flow="stack" className="gap-8">
      <Layout
        flow="stack"
        className="gap-3 md:flex-row md:items-center md:justify-between"
      >
        <Layout
          as="form"
          flow="stack"
          onSubmit={handleSearchSubmit}
          className="w-full gap-3 sm:flex-row sm:items-center md:max-w-2xl"
          role="search"
          aria-label="筛选公开课程"
        >
          <InputGroup className="flex-1">
            <InputGroupAddon align="inline-start">
              <SearchIcon />
            </InputGroupAddon>
            <InputGroupInput
              type="search"
              placeholder="搜索课程标题"
              aria-label="搜索课程标题"
              value={queryInput}
              onChange={(e) => setQueryInput(e.target.value)}
            />
            {queryInput ? (
              <InputGroupAddon align="inline-end">
                <InputGroupButton
                  aria-label="清空搜索词"
                  size="icon-xs"
                  onClick={handleClearSearch}
                >
                  <XIcon />
                </InputGroupButton>
              </InputGroupAddon>
            ) : null}
            <InputGroupAddon align="inline-end">
              <InputGroupButton type="submit" variant="secondary" size="xs">
                搜索
              </InputGroupButton>
            </InputGroupAddon>
          </InputGroup>

          <Select
            value={initialCategory || 'all'}
            onValueChange={handleCategoryChange}
          >
            <SelectTrigger
              className="w-full sm:w-48"
              aria-label="按分类筛选课程"
            >
              <SelectValue placeholder="全部分类" />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {COURSE_CATEGORIES.map((category) => (
                  <SelectItem key={category.slug} value={category.slug}>
                    {category.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Layout>

        <Layout
          flow="row"
          className="items-center gap-3 text-sm text-muted-foreground"
        >
          {!loadFailed && <Text>{total} 门课程</Text>}
          {hasActiveFilter ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={handleResetAll}
            >
              重置筛选
            </Button>
          ) : null}
        </Layout>
      </Layout>

      {loadFailed ? (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>课程目录加载失败</AlertTitle>
          <AlertDescription className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <Text>请稍后重试。</Text>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => router.refresh()}
            >
              重新加载
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      {!loadFailed && initialCourses.length === 0 ? (
        <Empty className="bg-canvas-soft py-16">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <BookOpenIcon />
            </EmptyMedia>
            <EmptyTitle>
              {hasActiveFilter ? '未找到匹配的课程' : '暂无公开课程'}
            </EmptyTitle>
            <EmptyDescription>
              {hasActiveFilter
                ? '试试其他标题或分类。'
                : '课程发布后会显示在这里。'}
            </EmptyDescription>
          </EmptyHeader>
          {hasActiveFilter ? (
            <EmptyContent>
              <Button
                type="button"
                variant="secondary"
                onClick={handleResetAll}
              >
                查看全部课程
              </Button>
            </EmptyContent>
          ) : null}
        </Empty>
      ) : null}

      {initialCourses.length > 0 ? (
        <Layout
          flow="grid"
          className="grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3"
        >
          {initialCourses.map((course) => {
            const isSalesClosed = course.saleState === 'sales_closed';
            const lessonsCount = course.lessonsCount ?? 0;

            return (
              <Card
                key={course.id}
                className="flex h-full flex-col justify-between transition-colors duration-120 hover:bg-secondary/60"
              >
                <CourseCover
                  category={course.category}
                  coverUrl={course.coverUrl}
                />
                <CardHeader className="gap-3">
                  <Layout flow="row" className="flex-wrap items-center gap-2">
                    <Badge variant="secondary">
                      {getCourseCategoryLabel(course.category)}
                    </Badge>
                    <Badge variant={course.isFree ? 'secondary' : 'default'}>
                      {course.isFree ? '免费公开' : '单课买断'}
                    </Badge>
                    {isSalesClosed ? (
                      <Badge variant="destructive">已停售</Badge>
                    ) : null}
                  </Layout>
                  <CardTitle className="line-clamp-2">
                    <Link
                      href={`/courses/${course.id}`}
                      className="outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                    >
                      {course.title}
                    </Link>
                  </CardTitle>
                  <CardDescription className="line-clamp-3">
                    {course.description?.trim() || '暂无课程简介'}
                  </CardDescription>
                </CardHeader>

                <CardContent className="flex flex-col gap-3">
                  <Layout
                    flow="row"
                    className="flex-wrap items-center justify-between gap-2 text-sm text-muted-foreground"
                  >
                    <Text className="inline-flex items-center gap-1.5">
                      <UserIcon className="size-3.5" />
                      <Text>{course.authorNickname?.trim() || '学屿讲师'}</Text>
                    </Text>
                    <Text className="inline-flex items-center gap-1.5">
                      <BookOpenIcon className="size-3.5" />
                      <Text>共 {lessonsCount} 课时</Text>
                    </Text>
                  </Layout>
                </CardContent>

                <CardFooter className="mt-auto items-center justify-between pt-1">
                  <Layout className="text-base font-medium text-foreground">
                    {formatPriceCny(course.priceAmountMinor, course.isFree)}
                  </Layout>
                  <Button asChild variant="secondary" size="sm">
                    <Link href={`/courses/${course.id}`}>
                      查看详情
                      <ArrowRightIcon data-icon="inline-end" />
                    </Link>
                  </Button>
                </CardFooter>
              </Card>
            );
          })}
        </Layout>
      ) : null}

      {totalPages > 1 ? (
        <Pagination>
          <PaginationContent>
            {page > 1 ? (
              <PaginationItem>
                <PaginationPrevious
                  text="上一页"
                  href={buildCoursesHref(
                    initialQuery,
                    initialCategory,
                    page - 1,
                  )}
                />
              </PaginationItem>
            ) : null}

            {pageNumbers.map((num) => (
              <PaginationItem key={num}>
                <PaginationLink
                  href={buildCoursesHref(initialQuery, initialCategory, num)}
                  isActive={num === page}
                >
                  {num}
                </PaginationLink>
              </PaginationItem>
            ))}

            {page < totalPages ? (
              <PaginationItem>
                <PaginationNext
                  text="下一页"
                  href={buildCoursesHref(
                    initialQuery,
                    initialCategory,
                    page + 1,
                  )}
                />
              </PaginationItem>
            ) : null}
          </PaginationContent>
        </Pagination>
      ) : null}
    </Layout>
  );
}
