export interface CategoryOption {
  slug: string;
  label: string;
  description: string;
}

export const COURSE_CATEGORIES: readonly CategoryOption[] = [
  {slug: 'all', label: '全部分类', description: '浏览全部已发布课程'},
  {
    slug: 'ai',
    label: 'AI 与智能体',
    description: '大模型应用、检索增强与 Agent 工程实践',
  },
  {
    slug: 'engineering',
    label: '软件工程',
    description: '全栈开发、架构设计、测试与交付实践',
  },
  {
    slug: 'product',
    label: '产品与协作',
    description: '需求分析、产品定义与项目推进方法',
  },
  {
    slug: 'design',
    label: '界面与设计系统',
    description: '交互设计、组件体系与可访问性实践',
  },
  {
    slug: 'general',
    label: '通用学习',
    description: '跨领域通识与学习方法课程',
  },
];

export const COMMUNITY_TOPICS: readonly CategoryOption[] = [
  {slug: 'all', label: '全部话题', description: '查看最新社区讨论'},
  {
    slug: 'learning-method',
    label: '学习方法',
    description: '拆解学习路径、笔记整理与高效复盘经验',
  },
  {
    slug: 'course-qa',
    label: '课程答疑',
    description: '围绕课程内容、案例实操和知识点提问交流',
  },
  {
    slug: 'project-practice',
    label: '项目实战',
    description: '分享工程落地、代码架构和真实排障过程',
  },
  {
    slug: 'resources',
    label: '资料整理',
    description: '整理可公开查阅的文档清单、工具与参考资料',
  },
  {
    slug: 'general',
    label: '综合讨论',
    description: '开放式学习交流与社区建议',
  },
];

export function getCourseCategoryLabel(slug?: string): string {
  if (!slug) {
    return '通用学习';
  }
  const found = COURSE_CATEGORIES.find((item) => item.slug === slug);
  return found ? found.label : slug;
}

export function getTopicMeta(slug?: string): CategoryOption {
  if (!slug) {
    return COMMUNITY_TOPICS[COMMUNITY_TOPICS.length - 1];
  }
  return (
    COMMUNITY_TOPICS.find((item) => item.slug === slug) ?? {
      slug,
      label: slug,
      description: '围绕该话题展开的公开学习讨论。',
    }
  );
}

export function formatPriceCny(amountMinor?: number, isFree?: boolean): string {
  if (isFree || amountMinor === 0) {
    return '免费';
  }
  if (typeof amountMinor !== 'number' || Number.isNaN(amountMinor)) {
    return '待定价';
  }
  const yuan = amountMinor / 100;
  return `¥${Number.isInteger(yuan) ? yuan.toFixed(0) : yuan.toFixed(2)}`;
}

export function formatDateTime(value?: string): string {
  if (!value) {
    return '刚刚';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date);
}

export function formatDateShort(value?: string): string {
  if (!value) {
    return '';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
  }).format(date);
}

/**
 * 校验并净化站内回跳路径，防止开放重定向到外部站点（R-24）。
 */
export function sanitizeNextPath(raw?: string | null): string {
  if (!raw) {
    return '/';
  }
  const trimmed = raw.trim();
  if (!trimmed.startsWith('/') || trimmed.startsWith('//')) {
    return '/';
  }
  if (trimmed.includes('\\') || trimmed.startsWith('/account')) {
    return '/';
  }
  return trimmed;
}

export function buildLoginHref(currentPath: string): string {
  const safe = sanitizeNextPath(currentPath);
  if (safe === '/') {
    return '/account';
  }
  return `/account?next=${encodeURIComponent(safe)}`;
}

export function getErrorMessage(
  err: unknown,
  fallback = '操作失败，请稍后重试',
): string {
  if (err && typeof err === 'object') {
    const response = (
      err as {
        response?: {data?: {message?: string; error?: string}};
        message?: string;
      }
    ).response;
    if (response?.data?.message) {
      return response.data.message;
    }
    if (response?.data?.error) {
      return response.data.error;
    }
    const msg = (err as {message?: string}).message;
    if (typeof msg === 'string' && msg.trim()) {
      return msg;
    }
  }
  return fallback;
}
