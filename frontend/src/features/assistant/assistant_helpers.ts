import {getErrorMessage} from '@/lib/format';

export interface ClassifiedAssistantError {
  title: string;
  description: string;
  retryable: boolean;
  kind:
    | 'auth'
    | 'verify'
    | 'quota'
    | 'search'
    | 'config'
    | 'revoked'
    | 'cancelled'
    | 'generic';
}

export function formatCentsToYuan(cents?: number): string {
  if (typeof cents !== 'number' || Number.isNaN(cents)) {
    return '—';
  }
  return `¥${(cents / 100).toFixed(2)}`;
}

export function formatPricePerK(pricePerK?: number, currency?: string): string {
  if (typeof pricePerK !== 'number' || Number.isNaN(pricePerK)) {
    return '—';
  }
  const unit = currency && currency.toUpperCase() !== 'CNY' ? currency : '分';
  return `${pricePerK} ${unit}/千Token`;
}

export function getProviderLabel(providerId?: string): string {
  switch ((providerId ?? '').toLowerCase()) {
    case 'deepseek':
      return 'DeepSeek';
    case 'openrouter':
      return 'OpenRouter';
    case 'volcengine':
      return '火山引擎';
    case 'openai':
      return 'OpenAI';
    default:
      return providerId || '未指定';
  }
}

export function getAgentStateLabel(state?: Api.AgentState): string {
  switch (state) {
    case 'published':
      return '已发布';
    case 'disabled':
      return '已停用';
    case 'draft':
    default:
      return '草稿';
  }
}

export function getKnowledgeVisibilityLabel(
  visibility?: Api.Visibility,
): string {
  switch (visibility) {
    case 'public':
      return '公开可见';
    case 'private':
      return '私有受限';
    default:
      return visibility ?? '未设置';
  }
}

export function getSourceKindLabel(kind?: Api.SourceKind): string {
  switch (kind) {
    case 'course_text':
      return '授权课程图文';
    case 'manual_doc':
      return '手动文档';
    default:
      return kind ?? '未知类型';
  }
}

export function getSourceStateLabel(state?: Api.SourceState): string {
  switch (state) {
    case 'draft':
      return '待切片索引';
    case 'indexing':
      return '切片索引中';
    case 'indexed':
      return '已就绪待发布';
    case 'published':
      return '已发布可检索';
    case 'revoked':
      return '已撤回停用';
    default:
      return state ?? '未知状态';
  }
}

export function getIndexJobStateLabel(state?: string): string {
  switch (state) {
    case 'running':
      return '执行中';
    case 'success':
      return '索引已完成';
    case 'failed':
      return '执行失败';
    case 'paused_budget':
      return '因后台预算暂停';
    default:
      return state ?? '未知状态';
  }
}

export function getIndexJobStageLabel(stage?: string): string {
  switch (stage) {
    case 'budget_admission':
      return '后台预算准入校验';
    case 'chunking':
      return '文档切片处理';
    case 'indexing':
      return '索引写入';
    case 'completed':
      return '全部完成';
    case 'failed':
      return '处理中断';
    default:
      return stage ?? '未知阶段';
  }
}

export function getReconciliationStateLabel(
  state?: Api.ReconciliationState,
): string {
  switch (state) {
    case 'pending':
      return '待自动核对';
    case 'manual_review':
      return '待人工核对';
    case 'resolved':
      return '已核对结清';
    default:
      return state ?? '未知状态';
  }
}

export function getPurposeLabel(purpose?: Api.ReservationPurpose): string {
  switch (purpose) {
    case 'agent_chat':
      return '智能问答';
    case 'index_job':
      return '知识入库索引';
    default:
      return purpose ?? '未知用途';
  }
}

export function isModelExpired(expiresAt?: string): boolean {
  if (!expiresAt) {
    return false;
  }
  const ts = Date.parse(expiresAt);
  if (Number.isNaN(ts)) {
    return false;
  }
  return ts <= Date.now();
}

export function computeUsagePercentage(used?: number, limit?: number): number {
  if (
    typeof used !== 'number' ||
    typeof limit !== 'number' ||
    limit <= 0 ||
    Number.isNaN(used) ||
    Number.isNaN(limit)
  ) {
    return 0;
  }
  return Math.min(100, Math.max(0, Math.round((used / limit) * 100)));
}

export function createIdempotencyKey(): string {
  if (
    typeof crypto !== 'undefined' &&
    typeof crypto.randomUUID === 'function'
  ) {
    return crypto.randomUUID();
  }
  return `ask_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
}

interface HttpErrorLike {
  code?: string;
  name?: string;
  response?: {
    status?: number;
    data?: {
      message?: string;
      error?: string;
    };
  };
}

export function extractBudgetPausedJob(err: unknown): Api.IndexJob | null {
  if (!err || typeof err !== 'object') {
    return null;
  }
  const response = (err as {response?: {status?: number; data?: Api.IndexJob}})
    .response;
  if (
    response?.status === 402 &&
    response.data &&
    typeof response.data === 'object'
  ) {
    return response.data;
  }
  return null;
}

export function classifyAssistantError(err: unknown): ClassifiedAssistantError {
  const httpErr = (err && typeof err === 'object' ? err : {}) as HttpErrorLike;
  const status = httpErr.response?.status;
  const rawMessage = getErrorMessage(err, '');
  const lower = rawMessage.toLowerCase();

  if (
    httpErr.code === 'ERR_CANCELED' ||
    httpErr.name === 'CanceledError' ||
    httpErr.name === 'AbortError' ||
    lower.includes('cancelled') ||
    lower.includes('canceled')
  ) {
    return {
      title: '已取消本次运行',
      description: '您可以调整问题内容后重新提问，之前的输入已为您保留。',
      retryable: true,
      kind: 'cancelled',
    };
  }

  if (status === 401 || lower.includes('unauthorized')) {
    return {
      title: '请先登录账号',
      description: '登录状态已失效，请重新登录后继续使用学屿助手。',
      retryable: false,
      kind: 'auth',
    };
  }

  if (
    status === 403 &&
    (lower.includes('email') || lower.includes('verified'))
  ) {
    return {
      title: '请先完成邮箱验证',
      description: '验证邮箱后即可使用学屿助手提问并保留历史对话。',
      retryable: false,
      kind: 'verify',
    };
  }

  if (
    lower.includes('revoked') ||
    lower.includes('citation') ||
    lower.includes('entitlement') ||
    (status === 403 && lower.includes('access'))
  ) {
    return {
      title: '知识来源或课程权限已变更',
      description:
        '该会话引用的部分知识来源已被撤回或课程访问权限已失效，请刷新会话或切换其他问题重试。',
      retryable: true,
      kind: 'revoked',
    };
  }

  if (
    status === 429 ||
    lower.includes('rate limit') ||
    lower.includes('concurrency')
  ) {
    if (lower.includes('daily')) {
      return {
        title: '今日问答次数已达上限',
        description: '您的账号今日成功问答次数已用完，请明日再试。',
        retryable: false,
        kind: 'quota',
      };
    }
    if (lower.includes('concurrency')) {
      return {
        title: '当前已有进行中的问答',
        description: '请等待当前回答完成或取消运行后再发起新的提问。',
        retryable: true,
        kind: 'quota',
      };
    }
    return {
      title: '提问过于频繁',
      description: '短时间内请求次数过多，请稍等片刻后重试。',
      retryable: true,
      kind: 'quota',
    };
  }

  if (status === 402 || lower.includes('budget')) {
    return {
      title: 'AI 预算额度受限',
      description:
        '当前账号或平台的 AI 预算额度已达上限，暂无法发起新的问答。请稍后再试或联系管理员调整预算。',
      retryable: false,
      kind: 'quota',
    };
  }

  if (
    status === 503 ||
    lower.includes('paused') ||
    lower.includes('not configured') ||
    lower.includes('price expired') ||
    lower.includes('disabled')
  ) {
    if (lower.includes('paused')) {
      return {
        title: '平台已暂停新增 AI 请求',
        description: '管理员已临时暂停平台 AI 问答服务，请稍后关注恢复通知。',
        retryable: false,
        kind: 'config',
      };
    }
    if (lower.includes('disabled')) {
      return {
        title: '当前助手实例已停用',
        description: '请切换到其他已发布的学屿助手实例后再继续提问。',
        retryable: false,
        kind: 'config',
      };
    }
    return {
      title: '助手服务配置暂不可用',
      description:
        '当前模型目录、价目有效期或搜索服务配置尚未就绪，请联系管理员检查模型与费用配置。',
      retryable: true,
      kind: 'config',
    };
  }

  if (status === 502 || lower.includes('search') || lower.includes('timeout')) {
    return {
      title: '实时联网搜索或模型生成失败',
      description:
        '联网搜索或模型响应暂时不可用，本次未扣除成功次数。您的输入已完整保留，可直接重试。',
      retryable: true,
      kind: 'search',
    };
  }

  return {
    title: '提问未能完成',
    description: '服务处理请求时出现异常，您的输入已完整保留，请稍后点击重试。',
    retryable: true,
    kind: 'generic',
  };
}
