export type CourseBadgeVariant = 'default' | 'secondary' | 'destructive';

export interface CourseStatusMeta {
  label: string;
  variant: CourseBadgeVariant;
  description: string;
}

export function getProductionStateMeta(state?: string): CourseStatusMeta {
  switch (state) {
    case 'approved':
      return {
        label: '已发布',
        variant: 'default',
        description: '已通过审核并对公开目录及学员可见',
      };
    case 'pending_review':
      return {
        label: '待审核',
        variant: 'secondary',
        description: '已提交管理员审核，审核通过后将对外发布',
      };
    case 'rejected':
      return {
        label: '已退回',
        variant: 'destructive',
        description: '管理员审核未通过，请根据退回说明修改后重新提交',
      };
    case 'draft':
    default:
      return {
        label: '草稿',
        variant: 'secondary',
        description: '仅讲师本人与管理员可见，完善内容后可提交审核',
      };
  }
}

export function getSaleStateMeta(state?: string): CourseStatusMeta {
  if (state === 'sales_closed') {
    return {
      label: '已停售',
      variant: 'secondary',
      description: '已停止新购与新加入，已获授权学员可继续学习',
    };
  }
  return {
    label: '在售中',
    variant: 'secondary',
    description: '正常开放加入或购买',
  };
}

export function getDeliveryStateMeta(state?: string): CourseStatusMeta {
  if (state === 'withdrawn') {
    return {
      label: '已撤回',
      variant: 'destructive',
      description: '课程已因合规原因撤回交付，暂不可访问或购买',
    };
  }
  return {
    label: '正常交付',
    variant: 'secondary',
    description: '内容可正常交付学习',
  };
}

export function getCourseCombinedStatusMeta(
  course: Pick<
    LearningApi.CourseResponse,
    'productionState' | 'saleState' | 'deliveryState'
  >,
): CourseStatusMeta {
  if (course.deliveryState === 'withdrawn') {
    return getDeliveryStateMeta('withdrawn');
  }
  if (course.saleState === 'sales_closed') {
    return getSaleStateMeta('sales_closed');
  }
  return getProductionStateMeta(course.productionState);
}

export interface FlattenedLesson {
  sectionId: string;
  sectionTitle: string;
  sectionIndex: number;
  lessonIndex: number;
  lesson: LearningApi.Lesson;
}

export function sortSectionsAndLessons(
  sections?: LearningApi.Section[],
): LearningApi.Section[] {
  if (!Array.isArray(sections)) {
    return [];
  }
  return [...sections]
    .sort((a, b) => (a.sortOrder ?? 0) - (b.sortOrder ?? 0))
    .map((section) => ({
      ...section,
      lessons: Array.isArray(section.lessons)
        ? [...section.lessons].sort(
            (a, b) => (a.sortOrder ?? 0) - (b.sortOrder ?? 0),
          )
        : [],
    }));
}

export function flattenCourseLessons(
  sections?: LearningApi.Section[],
): FlattenedLesson[] {
  const sorted = sortSectionsAndLessons(sections);
  const result: FlattenedLesson[] = [];
  sorted.forEach((section, sectionIndex) => {
    (section.lessons ?? []).forEach((lesson, lessonIndex) => {
      if (lesson.id) {
        result.push({
          sectionId: section.id ?? `sec-${sectionIndex}`,
          sectionTitle: section.title ?? `第 ${sectionIndex + 1} 章`,
          sectionIndex,
          lessonIndex,
          lesson,
        });
      }
    });
  });
  return result;
}

export function countCourseLessons(
  course: Pick<LearningApi.CourseResponse, 'lessonCount' | 'sections'>,
): number {
  if (typeof course.lessonCount === 'number' && course.lessonCount >= 0) {
    return course.lessonCount;
  }
  return (course.sections ?? []).reduce(
    (sum, section) => sum + (section.lessons?.length ?? 0),
    0,
  );
}

export function formatDurationSeconds(seconds?: number): string {
  if (!seconds || seconds <= 0) {
    return '0 秒';
  }
  const total = Math.floor(seconds);
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  if (hours > 0) {
    return `${hours} 小时 ${minutes} 分`;
  }
  if (minutes > 0) {
    return `${minutes} 分 ${secs} 秒`;
  }
  return `${secs} 秒`;
}

export function getHttpStatus(err: unknown): number | undefined {
  if (err && typeof err === 'object') {
    const response = (err as {response?: {status?: number}}).response;
    if (typeof response?.status === 'number') {
      return response.status;
    }
  }
  return undefined;
}
