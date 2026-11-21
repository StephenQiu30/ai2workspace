import type {RequestOptions} from '@/lib/request/transport';
import {getAssistantHealth} from '@/services/generated/assistant/health';
import {getCommerceHealth} from '@/services/generated/commerce/health';
import {getCommunityHealth} from '@/services/generated/community/health';
import {getIdentityHealth} from '@/services/generated/identity/health';
import {getLearningHealth} from '@/services/generated/learning/health';

export type ServiceId =
  'identity' | 'learning' | 'commerce' | 'community' | 'assistant';

export interface ServiceEntryLink {
  label: string;
  href: string;
}

export interface ServiceMetadata {
  id: ServiceId;
  title: string;
  description: string;
  port: number;
  healthPath: string;
  swaggerPath: string;
  routes: readonly ServiceEntryLink[];
}

export interface HealthPayload {
  service?: string;
  status?: string;
}

export interface ServiceHealthRecord {
  id: ServiceId;
  available: boolean;
  reportedService: string | null;
  status: string;
  latencyMs: number | null;
  checkedAt: string;
}

export interface ServiceHealthSummary {
  total: number;
  healthy: number;
  unhealthy: number;
  allHealthy: boolean;
}

export type HealthFetcher = (
  options?: RequestOptions,
) => Promise<HealthPayload>;

export const SERVICE_CATALOG: readonly ServiceMetadata[] = [
  {
    id: 'identity',
    title: '账号与访问控制',
    description: '账号注册、邮箱验证、登录会话与讲师/管理员权限控制。',
    port: 3123,
    healthPath: '/api/identity/health',
    swaggerPath: '/swagger/identity/index.html',
    routes: [
      {label: '账号中心', href: '/account'},
      {label: '账号管理', href: '/manage/accounts'},
    ],
  },
  {
    id: 'learning',
    title: '课程与媒体学习',
    description: '课程供给与审核发布、受控视频上传处理、播放鉴权与学习进度。',
    port: 3124,
    healthPath: '/api/learning/health',
    swaggerPath: '/swagger/learning/index.html',
    routes: [
      {label: '课程广场', href: '/courses'},
      {label: '课程管理', href: '/manage/courses'},
    ],
  },
  {
    id: 'commerce',
    title: '交易、退款与权益',
    description: '课程定价、订单与支付确认、购买权益校验及 7×24h 退款治理。',
    port: 3125,
    healthPath: '/api/commerce/health',
    swaggerPath: '/swagger/commerce/index.html',
    routes: [{label: '我的订单', href: '/orders'}],
  },
  {
    id: 'community',
    title: '社区互动与内容治理',
    description: '全站与课程讨论帖子、单层评论、互斥赞踩、收藏举报与站内通知。',
    port: 3126,
    healthPath: '/api/community/health',
    swaggerPath: '/swagger/community/index.html',
    routes: [
      {label: '讨论社区', href: '/community'},
      {label: '站内通知', href: '/notifications'},
    ],
  },
  {
    id: 'assistant',
    title: 'Agent、知识库与 AI 治理',
    description: '知识库切片检索、AI 费用原子准入治理与通用 Agent 联网问答。',
    port: 3127,
    healthPath: '/api/assistant/health',
    swaggerPath: '/swagger/assistant/index.html',
    routes: [
      {label: '知识库管理', href: '/manage/knowledge'},
      {label: 'Agent 管理', href: '/manage/agents'},
      {label: 'AI 费用治理', href: '/manage/ai-governance'},
    ],
  },
];

const DEFAULT_FETCHERS: Record<ServiceId, HealthFetcher> = {
  identity: getIdentityHealth,
  learning: getLearningHealth,
  commerce: getCommerceHealth,
  community: getCommunityHealth,
  assistant: getAssistantHealth,
};

export function createUnavailableRecord(
  id: ServiceId,
  checkedAt: string = new Date().toISOString(),
): ServiceHealthRecord {
  return {
    id,
    available: false,
    reportedService: null,
    status: 'unavailable',
    latencyMs: null,
    checkedAt,
  };
}

export async function checkServiceHealth(
  id: ServiceId,
  options?: RequestOptions,
  fetchers: Record<ServiceId, HealthFetcher> = DEFAULT_FETCHERS,
): Promise<ServiceHealthRecord> {
  const startedAt = Date.now();
  try {
    const payload = await fetchers[id](options);
    const latencyMs = Math.max(0, Date.now() - startedAt);
    const available = payload?.status === 'ok';
    return {
      id,
      available,
      reportedService: payload?.service ?? null,
      status: payload?.status ?? 'unknown',
      latencyMs,
      checkedAt: new Date().toISOString(),
    };
  } catch {
    return createUnavailableRecord(id);
  }
}

export async function checkAllServicesHealth(
  options?: RequestOptions,
  fetchers: Record<ServiceId, HealthFetcher> = DEFAULT_FETCHERS,
): Promise<ServiceHealthRecord[]> {
  return Promise.all(
    SERVICE_CATALOG.map((service) =>
      checkServiceHealth(service.id, options, fetchers),
    ),
  );
}

export function summarizeServiceHealth(
  records: readonly ServiceHealthRecord[],
): ServiceHealthSummary {
  const total = records.length;
  const healthy = records.filter((record) => record.available).length;
  const unhealthy = total - healthy;
  return {
    total,
    healthy,
    unhealthy,
    allHealthy: total > 0 && unhealthy === 0,
  };
}

export function formatCheckedTime(isoTimestamp: string): string {
  const date = new Date(isoTimestamp);
  if (Number.isNaN(date.getTime())) {
    return '--:--:--';
  }
  return new Intl.DateTimeFormat('zh-CN', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    timeZone: 'Asia/Shanghai',
  }).format(date);
}
