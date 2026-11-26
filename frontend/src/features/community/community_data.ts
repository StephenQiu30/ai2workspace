import 'server-only';
import type {SessionUser} from '@/lib/auth/roles';
import {COMMUNITY_TOPICS, getErrorMessage} from '@/lib/format';
import {getServerRequestOptions} from '@/lib/request/server';
import {commerceGetCoursePrice} from '@/services/generated/api/commerce';
import {
  communityListPosts,
  communityListTopics,
} from '@/services/generated/api/community';
import {
  learningListCourses,
  learningListMyCourses,
} from '@/services/generated/api/courses';

export interface HomeCourseItem extends Api.CourseResponse {
  priceAmountMinor?: number;
}

export interface TopicSummary {
  slug: string;
  name: string;
  description: string;
  postCount: number;
}

export interface HomePageData {
  activeTopic: string;
  page: number;
  pageSize: number;
  courses: HomeCourseItem[];
  coursesError: string | null;
  posts: Api.PostResponse[];
  postsTotal: number;
  services: Api.PostResponse[];
  servicesError: string | null;
  postsError: string | null;
  topics: TopicSummary[];
  myCourses: Api.MyCourseResponse[];
  myCoursesError: string | null;
}

export interface TopicPageData {
  topic: TopicSummary | null;
  topics: TopicSummary[];
  posts: Api.PostResponse[];
  postsTotal: number;
  page: number;
  pageSize: number;
  postsError: string | null;
}

export const HOME_POST_PAGE_SIZE = 10;
export const TOPIC_POST_PAGE_SIZE = 10;
export const HOME_COURSE_LIMIT = 6;

const VALID_TOPIC_SLUGS = new Set(
  COMMUNITY_TOPICS.filter((item) => item.slug !== 'all').map(
    (item) => item.slug,
  ),
);

export function parsePositivePage(raw?: string | string[]): number {
  const value = Array.isArray(raw) ? raw[0] : raw;
  const parsed = Number.parseInt(value ?? '1', 10);
  if (Number.isNaN(parsed) || parsed < 1) {
    return 1;
  }
  return parsed;
}

export function normalizeTopicParam(raw?: string | string[]): string {
  const value = (Array.isArray(raw) ? raw[0] : raw)?.trim() ?? '';
  if (!value || value === 'all') {
    return 'all';
  }
  return VALID_TOPIC_SLUGS.has(value) ? value : 'all';
}

function mergeTopics(apiTopics?: Api.TopicInfo[]): TopicSummary[] {
  const bySlug = new Map<string, Api.TopicInfo>();
  for (const item of apiTopics ?? []) {
    if (item.slug) {
      bySlug.set(item.slug, item);
    }
  }
  return COMMUNITY_TOPICS.filter((item) => item.slug !== 'all').map((meta) => {
    const remote = bySlug.get(meta.slug);
    return {
      slug: meta.slug,
      name: remote?.name?.trim() || meta.label,
      description: remote?.description?.trim() || meta.description,
      postCount: typeof remote?.postCount === 'number' ? remote.postCount : 0,
    };
  });
}

async function enrichCoursePrices(
  courses: Api.CourseResponse[],
  options: Awaited<ReturnType<typeof getServerRequestOptions>>,
): Promise<HomeCourseItem[]> {
  if (courses.length === 0) {
    return [];
  }
  const results = await Promise.all(
    courses.map(async (course): Promise<HomeCourseItem> => {
      if (course.isFree || !course.id) {
        return {...course, priceAmountMinor: 0};
      }
      try {
        const price = await commerceGetCoursePrice({id: course.id}, options);
        return {
          ...course,
          priceAmountMinor: price.amountMinor,
        };
      } catch {
        return {...course};
      }
    }),
  );
  return results;
}

export async function loadHomePageData(params: {
  topic?: string | string[];
  page?: string | string[];
  user: SessionUser | null;
}): Promise<HomePageData> {
  const activeTopic = normalizeTopicParam(params.topic);
  const page = parsePositivePage(params.page);
  const pageSize = HOME_POST_PAGE_SIZE;

  let options: Awaited<ReturnType<typeof getServerRequestOptions>>;
  try {
    options = await getServerRequestOptions();
  } catch (err: unknown) {
    const message = getErrorMessage(err, '服务端接口地址未配置');
    return {
      activeTopic,
      page,
      pageSize,
      courses: [],
      coursesError: message,
      posts: [],
      postsTotal: 0,
      services: [],
      servicesError: message,
      postsError: message,
      topics: mergeTopics(),
      myCourses: [],
      myCoursesError: params.user ? message : null,
    };
  }

  const fetchPosts = (
    topic: string | undefined,
    page: number,
    pageSize: number,
  ) =>
    communityListPosts({scope: 'global', topic, page, pageSize}, options)
      .then((data) => ({ok: true as const, data}))
      .catch((err: unknown) => ({
        ok: false as const,
        error: getErrorMessage(err, '社区内容暂时无法加载'),
      }));
  const feed = fetchPosts(
    activeTopic === 'all' ? undefined : activeTopic,
    page,
    pageSize,
  );
  const serviceList =
    activeTopic === 'services' && page === 1
      ? feed
      : fetchPosts('services', 1, 4);

  const [
    coursesResult,
    postsResult,
    topicsResult,
    myCoursesResult,
    servicesResult,
  ] = await Promise.all([
    learningListCourses({page: 1, pageSize: HOME_COURSE_LIMIT}, options)
      .then((data) => ({ok: true as const, data}))
      .catch((err: unknown) => ({
        ok: false as const,
        error: getErrorMessage(err, '课程列表暂时无法加载，请稍后重试'),
      })),
    feed,
    communityListTopics(options)
      .then((data) => ({ok: true as const, data}))
      .catch(() => ({ok: false as const, data: undefined})),
    params.user
      ? learningListMyCourses(options)
          .then((data) => ({ok: true as const, data}))
          .catch((err: unknown) => ({
            ok: false as const,
            error: getErrorMessage(err, '学习进度暂时无法加载'),
          }))
      : Promise.resolve({
          ok: true as const,
          data: [] as Api.MyCourseResponse[],
        }),
    serviceList,
  ]);

  const rawCourses = coursesResult.ok ? (coursesResult.data.courses ?? []) : [];
  const courses = await enrichCoursePrices(rawCourses, options);

  const topics = mergeTopics(topicsResult.ok ? topicsResult.data.items : []);

  return {
    activeTopic,
    page,
    pageSize,
    courses,
    coursesError: coursesResult.ok ? null : coursesResult.error,
    posts: postsResult.ok ? (postsResult.data.items ?? []) : [],
    postsTotal: postsResult.ok ? (postsResult.data.total ?? 0) : 0,
    services: servicesResult.ok
      ? (servicesResult.data.items ?? [])
          .filter((post) => post.id && post.title && post.topic === 'services')
          .slice(0, 4)
      : [],
    servicesError: servicesResult.ok ? null : servicesResult.error,
    postsError: postsResult.ok ? null : postsResult.error,
    topics,
    myCourses: myCoursesResult.ok ? myCoursesResult.data : [],
    myCoursesError: myCoursesResult.ok ? null : myCoursesResult.error,
  };
}

export async function loadTopicPageData(params: {
  slug: string;
  page?: string | string[];
}): Promise<TopicPageData> {
  const slug = params.slug.trim();
  const page = parsePositivePage(params.page);
  const pageSize = TOPIC_POST_PAGE_SIZE;

  if (!VALID_TOPIC_SLUGS.has(slug)) {
    return {
      topic: null,
      topics: mergeTopics(),
      posts: [],
      postsTotal: 0,
      page,
      pageSize,
      postsError: null,
    };
  }

  let options: Awaited<ReturnType<typeof getServerRequestOptions>>;
  try {
    options = await getServerRequestOptions();
  } catch (err: unknown) {
    const topics = mergeTopics();
    return {
      topic: topics.find((item) => item.slug === slug) ?? null,
      topics,
      posts: [],
      postsTotal: 0,
      page,
      pageSize,
      postsError: getErrorMessage(err, '服务端接口地址未配置'),
    };
  }

  const [postsResult, topicsResult] = await Promise.all([
    communityListPosts(
      {
        scope: 'global',
        topic: slug,
        page,
        pageSize,
      },
      options,
    )
      .then((data) => ({ok: true as const, data}))
      .catch((err: unknown) => ({
        ok: false as const,
        error: getErrorMessage(err, '话题讨论列表暂时无法加载，请稍后重试'),
      })),
    communityListTopics(options)
      .then((data) => ({ok: true as const, data}))
      .catch(() => ({ok: false as const, data: undefined})),
  ]);

  const topics = mergeTopics(topicsResult.ok ? topicsResult.data.items : []);
  const topic = topics.find((item) => item.slug === slug) ?? null;

  return {
    topic,
    topics,
    posts: postsResult.ok ? (postsResult.data.items ?? []) : [],
    postsTotal: postsResult.ok ? (postsResult.data.total ?? 0) : 0,
    page,
    pageSize,
    postsError: postsResult.ok ? null : postsResult.error,
  };
}
