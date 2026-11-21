'use client';

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from 'react';
import Link from 'next/link';
import {usePathname} from 'next/navigation';
import {
  AlertCircleIcon,
  ArrowUpIcon,
  BookOpenIcon,
  ExternalLinkIcon,
  GlobeIcon,
  HistoryIcon,
  MailCheckIcon,
  PlusIcon,
  RefreshCwIcon,
  ShieldAlertIcon,
  SquareIcon,
  XIcon,
} from 'lucide-react';
import {cn} from 'cn';
import {LearnisLogo} from '@/components/brand/learnis_logo';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
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
  InputGroupText,
  InputGroupTextarea,
} from '@/components/ui/input-group';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import {Spinner} from '@/components/ui/spinner';
import {useAssistantContext} from '@/features/assistant/assistant_context';
import {
  classifyAssistantError,
  createIdempotencyKey,
  type ClassifiedAssistantError,
} from '@/features/assistant/assistant_helpers';
import type {SessionUser} from '@/lib/auth/roles';
import {buildLoginHref, formatDateTime} from '@/lib/format';
import {assistantListAgents} from '@/services/generated/assistant/agents';
import {
  assistantAskQuestion,
  assistantCancelRun,
  assistantCreateSession,
  assistantListSessionMessages,
  assistantListSessions,
} from '@/services/generated/assistant/chat';
import {assistantGetGovernanceSummary} from '@/services/generated/assistant/governance';

export interface FloatingAssistantProps {
  initialUser: SessionUser | null;
}

type EvidenceDetail =
  | {kind: 'citation'; data: AssistantApi.Citation}
  | {kind: 'web'; data: AssistantApi.SearchResultItem};

function isSafeExternalHref(url?: string): boolean {
  if (!url) {
    return false;
  }
  try {
    const parsed = new URL(url);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:';
  } catch {
    return false;
  }
}

function getSessionLabel(session: AssistantApi.Session): string {
  const title = session.title?.trim();
  if (title) {
    return title;
  }
  return session.courseId ? '课程上下文会话' : '学屿助手会话';
}

export function FloatingAssistant({initialUser}: FloatingAssistantProps) {
  const pathname = usePathname() || '/';
  const {open, setOpen, courseContext, setCourseContext} =
    useAssistantContext();

  const user = initialUser;
  const isVerifiedUser = Boolean(user && user.emailVerified);

  const [isMobile, setIsMobile] = useState(false);
  const [agents, setAgents] = useState<AssistantApi.Configuration[]>([]);
  const [selectedSlug, setSelectedSlug] = useState<string>('');
  const [sessions, setSessions] = useState<AssistantApi.Session[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string>('');
  const [messages, setMessages] = useState<AssistantApi.Message[]>([]);
  const [summary, setSummary] = useState<AssistantApi.UsageSummary | null>(
    null,
  );

  const [bootstrapping, setBootstrapping] = useState(false);
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [inputVal, setInputVal] = useState('');
  const [asking, setAsking] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [activeRunId, setActiveRunId] = useState<string>('');
  const [lastError, setLastError] = useState<ClassifiedAssistantError | null>(
    null,
  );
  const [evidenceDetail, setEvidenceDetail] = useState<EvidenceDetail | null>(
    null,
  );

  const triggerRef = useRef<HTMLButtonElement>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  const isComposingRef = useRef(false);
  const wasOpenRef = useRef(false);
  const hasBootstrappedRef = useRef(false);

  const publishedAgents = useMemo(
    () =>
      agents.filter(
        (item) =>
          Boolean(item.slug) &&
          item.state !== 'draft' &&
          item.state !== 'disabled',
      ),
    [agents],
  );

  const activeAgent = useMemo(
    () => publishedAgents.find((item) => item.slug === selectedSlug) ?? null,
    [publishedAgents, selectedSlug],
  );

  const remainingQuestions = useMemo(() => {
    if (
      !summary ||
      typeof summary.maxDailyQuestions !== 'number' ||
      typeof summary.questionsCount !== 'number'
    ) {
      return null;
    }
    return Math.max(0, summary.maxDailyQuestions - summary.questionsCount);
  }, [summary]);

  // Responsive viewport detection for desktop popover vs mobile full-screen Sheet
  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    const media = window.matchMedia('(max-width: 767px)');
    const update = () => setIsMobile(media.matches);
    update();
    media.addEventListener('change', update);
    return () => media.removeEventListener('change', update);
  }, []);

  // Restore focus to trigger button when floating panel closes
  useEffect(() => {
    if (wasOpenRef.current && !open) {
      window.requestAnimationFrame(() => {
        triggerRef.current?.focus();
      });
    }
    wasOpenRef.current = open;
  }, [open]);

  // Escape key closes desktop panel (when evidence detail dialog is not open)
  useEffect(() => {
    if (!open || isMobile) {
      return;
    }
    const handleKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape' && !evidenceDetail) {
        event.preventDefault();
        setOpen(false);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [open, isMobile, evidenceDetail, setOpen]);

  const refreshSummary = useCallback(async () => {
    if (!isVerifiedUser) {
      return;
    }
    try {
      const next = await assistantGetGovernanceSummary();
      if (next) {
        setSummary(next);
      }
    } catch {
      // Non-blocking summary refresh
    }
  }, [isVerifiedUser]);

  const loadSessionMessages = useCallback(async (sessionId: string) => {
    if (!sessionId) {
      setMessages([]);
      return;
    }
    setLoadingMessages(true);
    setLastError(null);
    try {
      const history = await assistantListSessionMessages({id: sessionId});
      setMessages(Array.isArray(history) ? history : []);
    } catch (err: unknown) {
      setLastError(classifyAssistantError(err));
    } finally {
      setLoadingMessages(false);
    }
  }, []);

  const bootstrapAssistant = useCallback(async () => {
    if (!isVerifiedUser) {
      return;
    }
    setBootstrapping(true);
    setLastError(null);
    try {
      const [agentList, summaryData, sessionList] = await Promise.all([
        assistantListAgents(),
        assistantGetGovernanceSummary().catch(() => null),
        assistantListSessions().catch(() => [] as AssistantApi.Session[]),
      ]);

      const validAgents = Array.isArray(agentList) ? agentList : [];
      setAgents(validAgents);

      const available = validAgents.filter(
        (item) =>
          Boolean(item.slug) &&
          item.state !== 'draft' &&
          item.state !== 'disabled',
      );
      setSelectedSlug((prev) => {
        if (prev && available.some((item) => item.slug === prev)) {
          return prev;
        }
        if (courseContext?.courseId) {
          const tutor = available.find((item) => item.slug === 'course_tutor');
          if (tutor?.slug) {
            return tutor.slug;
          }
        }
        return available[0]?.slug ?? '';
      });

      if (summaryData) {
        setSummary(summaryData);
      }

      const validSessions = Array.isArray(sessionList) ? sessionList : [];
      setSessions(validSessions);
      hasBootstrappedRef.current = true;
    } catch (err: unknown) {
      setLastError(classifyAssistantError(err));
    } finally {
      setBootstrapping(false);
    }
  }, [isVerifiedUser, courseContext?.courseId]);

  // Load initial agents, quota, and sessions when opened for verified users
  useEffect(() => {
    if (!open || !isVerifiedUser) {
      return;
    }
    if (!hasBootstrappedRef.current) {
      void bootstrapAssistant();
    } else {
      void refreshSummary();
    }
  }, [open, isVerifiedUser, bootstrapAssistant, refreshSummary]);

  // Auto-scroll to latest message
  useEffect(() => {
    if (!open) {
      return;
    }
    messagesEndRef.current?.scrollIntoView({behavior: 'smooth'});
  }, [messages, asking, open]);

  const handleSelectSession = useCallback(
    async (session: AssistantApi.Session) => {
      if (!session.id) {
        return;
      }
      setActiveSessionId(session.id);
      if (session.agentId) {
        const matched = publishedAgents.find(
          (item) => item.id === session.agentId,
        );
        if (matched?.slug) {
          setSelectedSlug(matched.slug);
        }
      }
      await loadSessionMessages(session.id);
    },
    [publishedAgents, loadSessionMessages],
  );

  const handleStartNewSession = useCallback(() => {
    setActiveSessionId('');
    setMessages([]);
    setLastError(null);
    setActiveRunId('');
  }, []);

  const handleSwitchAgent = useCallback((slug: string) => {
    setSelectedSlug(slug);
    setActiveSessionId('');
    setMessages([]);
    setLastError(null);
    setActiveRunId('');
  }, []);

  const handleCancelRun = useCallback(async () => {
    if (!asking) {
      return;
    }
    setCancelling(true);
    const controller = abortControllerRef.current;
    if (controller) {
      controller.abort();
      abortControllerRef.current = null;
    }

    if (activeSessionId && activeRunId) {
      try {
        await assistantCancelRun({
          id: activeSessionId,
          runId: activeRunId,
        });
      } catch {
        // Ignore if run already terminated via request cancellation
      }
    }

    setAsking(false);
    setCancelling(false);
    setActiveRunId('');
    setLastError({
      title: '已取消本次运行',
      description: '您的提问内容仍保留在输入框中，可修改后重新发送。',
      retryable: true,
      kind: 'cancelled',
    });
  }, [asking, activeSessionId, activeRunId]);

  const handleSend = useCallback(async () => {
    const questionText = inputVal.trim();
    if (!questionText || asking || !isVerifiedUser) {
      return;
    }
    if (!selectedSlug || !activeAgent) {
      setLastError({
        title: '暂无可用助手实例',
        description: '当前没有已发布的可用助手实例，请刷新后重试。',
        retryable: false,
        kind: 'config',
      });
      return;
    }
    if (summary?.paused) {
      setLastError({
        title: '平台已暂停新增 AI 请求',
        description:
          summary.pauseReason?.trim() ||
          '管理员已临时暂停新增 AI 请求，请稍后再试。',
        retryable: false,
        kind: 'config',
      });
      return;
    }

    setAsking(true);
    setLastError(null);

    const controller = new AbortController();
    abortControllerRef.current = controller;

    const tempMessageId = `pending_${Date.now()}`;
    const optimisticUserMessage: AssistantApi.Message = {
      id: tempMessageId,
      sender: 'user',
      content: questionText,
      createdAt: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, optimisticUserMessage]);

    try {
      let targetSessionId = activeSessionId;
      if (!targetSessionId) {
        const titlePrefix = courseContext?.courseTitle
          ? `${courseContext.courseTitle} · `
          : '';
        const sessionTitle = `${titlePrefix}${questionText.slice(0, 20)}`;
        const created = await assistantCreateSession(
          {
            agentSlug: selectedSlug,
            courseId: courseContext?.courseId,
            title: sessionTitle,
          },
          {signal: controller.signal, timeout: 15000},
        );
        if (!created?.id) {
          throw new Error('会话创建失败');
        }
        targetSessionId = created.id;
        setActiveSessionId(created.id);
        setSessions((prev) => [
          created,
          ...prev.filter((item) => item.id !== created.id),
        ]);
      }

      const idempotencyKey = createIdempotencyKey();
      const reply = await assistantAskQuestion(
        {id: targetSessionId},
        {question: questionText, idempotencyKey},
        {signal: controller.signal, timeout: 60000},
      );

      if (reply?.runId) {
        setActiveRunId(reply.runId);
      }

      // Clear user input ONLY after receiving a successful response
      setInputVal('');

      if (reply) {
        setMessages((prev) => [...prev, reply]);
      } else {
        await loadSessionMessages(targetSessionId);
      }

      await refreshSummary();
    } catch (err: unknown) {
      // Remove optimistic message on failure while keeping inputVal intact
      setMessages((prev) => prev.filter((item) => item.id !== tempMessageId));
      setLastError(classifyAssistantError(err));
    } finally {
      if (abortControllerRef.current === controller) {
        abortControllerRef.current = null;
      }
      setAsking(false);
      setCancelling(false);
      setActiveRunId('');
    }
  }, [
    inputVal,
    asking,
    isVerifiedUser,
    selectedSlug,
    activeAgent,
    summary,
    activeSessionId,
    courseContext,
    loadSessionMessages,
    refreshSummary,
  ]);

  const handleTextareaKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (
      event.key === 'Enter' &&
      !event.shiftKey &&
      !event.nativeEvent.isComposing &&
      !isComposingRef.current
    ) {
      event.preventDefault();
      void handleSend();
    }
  };

  const renderPanelContent = () => (
    <div className="flex h-full min-h-0 flex-col bg-popover text-popover-foreground">
      {/* Panel Header */}
      <div className="flex shrink-0 items-center justify-between gap-2 px-4 pt-4 pb-3">
        <div className="flex min-w-0 items-center gap-2.5">
          <LearnisLogo variant="mark" tone="ink" markSize={20} />
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h2 className="truncate text-base leading-6 font-medium text-foreground">
                学屿助手
              </h2>
              {courseContext ? (
                <Badge
                  variant="secondary"
                  className="max-w-[160px] gap-1"
                  title={courseContext.courseTitle ?? courseContext.courseId}
                >
                  <BookOpenIcon className="size-3 shrink-0" />
                  <span className="truncate">
                    {courseContext.courseTitle || '当前课程'}
                  </span>
                  <button
                    type="button"
                    onClick={() => setCourseContext(null)}
                    aria-label="清除当前课程上下文"
                    className="ml-0.5 rounded-full hover:text-foreground"
                  >
                    <XIcon className="size-3" />
                  </button>
                </Badge>
              ) : null}
            </div>
            <p className="text-xs text-muted-foreground">
              关闭仅收起面板，不取消运行
            </p>
          </div>
        </div>

        <div className="flex shrink-0 items-center gap-1">
          {isVerifiedUser ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label="切换历史会话"
                  title="历史会话"
                >
                  <HistoryIcon />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-64">
                <DropdownMenuLabel>历史对话</DropdownMenuLabel>
                <DropdownMenuGroup>
                  <DropdownMenuItem onSelect={handleStartNewSession}>
                    <PlusIcon />
                    <span>开启新对话</span>
                  </DropdownMenuItem>
                </DropdownMenuGroup>
                <DropdownMenuSeparator />
                {sessions.length === 0 ? (
                  <div className="px-2.5 py-3 text-xs text-muted-foreground">
                    暂无历史对话记录
                  </div>
                ) : (
                  <DropdownMenuGroup>
                    {sessions.slice(0, 15).map((session) => {
                      const isCurrent = session.id === activeSessionId;
                      return (
                        <DropdownMenuItem
                          key={session.id}
                          onSelect={() => void handleSelectSession(session)}
                          className={cn(
                            'flex flex-col items-start gap-0.5',
                            isCurrent ? 'bg-secondary font-medium' : '',
                          )}
                        >
                          <span className="w-full truncate text-sm">
                            {getSessionLabel(session)}
                          </span>
                          <span className="text-xs text-muted-foreground">
                            {formatDateTime(
                              session.updatedAt ?? session.createdAt,
                            )}
                          </span>
                        </DropdownMenuItem>
                      );
                    })}
                  </DropdownMenuGroup>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}

          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="关闭学屿助手（仅收起面板，不取消运行）"
            title="收起面板（不取消运行）"
            onClick={() => setOpen(false)}
          >
            <XIcon />
          </Button>
        </div>
      </div>

      {/* Auth Gate: Guest */}
      {!user ? (
        <div className="flex flex-1 flex-col justify-center px-6 py-8">
          <Empty className="bg-canvas-soft py-8">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <LearnisLogo variant="mark" tone="ink" markSize={20} />
              </EmptyMedia>
              <EmptyTitle>登录后启用学屿助手</EmptyTitle>
              <EmptyDescription>
                学屿助手结合已授权的课程知识库与实时联网搜索提供可追溯来源的学习答疑，并自动保存你的历史会话。
              </EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <Button asChild className="w-full">
                <Link
                  href={buildLoginHref(pathname)}
                  onClick={() => setOpen(false)}
                >
                  前往登录账号
                </Link>
              </Button>
              <Button variant="secondary" asChild className="w-full">
                <Link
                  href="/account?mode=register"
                  onClick={() => setOpen(false)}
                >
                  注册新账号
                </Link>
              </Button>
            </EmptyContent>
          </Empty>
        </div>
      ) : !user.emailVerified ? (
        /* Auth Gate: Unverified Email */
        <div className="flex flex-1 flex-col justify-center gap-4 px-6 py-8">
          <Empty className="bg-canvas-soft py-8">
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <MailCheckIcon />
              </EmptyMedia>
              <EmptyTitle>验证邮箱后即可提问</EmptyTitle>
              <EmptyDescription>
                当前账号（{user.email || user.nickname}
                ）尚未完成邮箱验证。为保障额度与会话安全，请先前往账号设置完成验证。
              </EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <Button asChild className="w-full">
                <Link href="/account" onClick={() => setOpen(false)}>
                  前往验证邮箱
                </Link>
              </Button>
            </EmptyContent>
          </Empty>
        </div>
      ) : (
        /* Verified User Q&A Body */
        <>
          {/* Agent Switcher & Quota Strip */}
          <div className="flex shrink-0 flex-wrap items-center justify-between gap-2 bg-canvas-soft px-4 py-2.5">
            <div className="flex min-w-0 flex-1 items-center gap-1.5 overflow-x-auto">
              {bootstrapping ? (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <Spinner className="size-3.5" />
                  <span>正在加载助手实例…</span>
                </div>
              ) : publishedAgents.length > 0 ? (
                publishedAgents.map((agent) => {
                  const slug = agent.slug ?? '';
                  const isActive = slug === selectedSlug;
                  return (
                    <Button
                      key={agent.id ?? slug}
                      variant={isActive ? 'default' : 'secondary'}
                      size="xs"
                      className="shrink-0"
                      onClick={() => handleSwitchAgent(slug)}
                      disabled={asking}
                    >
                      {agent.name || slug}
                    </Button>
                  );
                })
              ) : (
                <span className="text-xs text-muted-foreground">
                  暂无已发布的助手实例
                </span>
              )}
            </div>

            {summary ? (
              <span className="shrink-0 text-xs text-muted-foreground">
                {remainingQuestions !== null &&
                typeof summary.maxDailyQuestions === 'number'
                  ? `今日剩余 ${remainingQuestions}/${summary.maxDailyQuestions} 次`
                  : `今日已用 ${summary.questionsCount ?? 0} 次`}
              </span>
            ) : null}
          </div>

          {/* Platform Paused Alert */}
          {summary?.paused ? (
            <div className="shrink-0 px-4 pt-3">
              <Alert variant="destructive">
                <ShieldAlertIcon />
                <AlertTitle>平台已暂停新增 AI 请求</AlertTitle>
                <AlertDescription>
                  {summary.pauseReason?.trim() ||
                    '管理员已临时开启保护暂停，当前仅可浏览历史对话。'}
                </AlertDescription>
              </Alert>
            </div>
          ) : null}

          {/* Messages Viewport */}
          <div className="flex-1 space-y-4 overflow-y-auto px-4 py-4">
            {!bootstrapping && publishedAgents.length === 0 ? (
              <Empty className="bg-canvas-soft py-8">
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    <AlertCircleIcon />
                  </EmptyMedia>
                  <EmptyTitle>当前暂无可用助手实例</EmptyTitle>
                  <EmptyDescription>
                    管理员尚未发布可用的学屿助手实例，或全部实例已停用。请稍后刷新重试。
                  </EmptyDescription>
                </EmptyHeader>
                <EmptyContent>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => void bootstrapAssistant()}
                  >
                    <RefreshCwIcon />
                    刷新状态
                  </Button>
                </EmptyContent>
              </Empty>
            ) : loadingMessages ? (
              <div className="flex items-center justify-center gap-2 py-12 text-sm text-muted-foreground">
                <Spinner />
                <span>正在加载历史消息并复核引用权限…</span>
              </div>
            ) : messages.length === 0 ? (
              <Empty className="bg-canvas-soft py-10">
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    <LearnisLogo variant="mark" tone="ink" markSize={20} />
                  </EmptyMedia>
                  <EmptyTitle>
                    {activeAgent?.name || '学屿助手'}已就绪
                  </EmptyTitle>
                  <EmptyDescription>
                    可围绕课程知识点、实践问题或平台使用提问。每次回答均结合授权知识库检索与实时联网搜索，并提供可点击核验的依据来源。
                  </EmptyDescription>
                </EmptyHeader>
              </Empty>
            ) : (
              messages.map((message, idx) => {
                const isUser = message.sender === 'user';
                const citations = Array.isArray(message.citations)
                  ? message.citations
                  : [];
                const webSources = Array.isArray(message.searchQueries)
                  ? message.searchQueries
                  : [];

                return (
                  <div
                    key={message.id ?? `msg_${idx}`}
                    className={cn(
                      'flex flex-col gap-2',
                      isUser ? 'items-end' : 'items-start',
                    )}
                  >
                    <div
                      className={cn(
                        'max-w-[88%] rounded-2xl px-3.5 py-2.5 text-sm leading-6 whitespace-pre-wrap break-words',
                        isUser
                          ? 'bg-primary text-primary-foreground'
                          : 'bg-canvas-soft text-foreground',
                      )}
                    >
                      {message.content}
                    </div>

                    {!isUser && message.redacted ? (
                      <Alert className="max-w-[92%] py-2.5">
                        <ShieldAlertIcon />
                        <AlertTitle className="text-xs">
                          部分引用来源已受保护隐藏
                        </AlertTitle>
                        <AlertDescription className="text-xs">
                          该回答涉及的知识来源已被撤回或您的课程访问权限已变更，系统已自动隐藏受保护内容。
                        </AlertDescription>
                      </Alert>
                    ) : null}

                    {!isUser &&
                    (citations.length > 0 || webSources.length > 0) ? (
                      <div className="flex max-w-[94%] flex-col gap-2">
                        {citations.length > 0 ? (
                          <div className="flex flex-wrap items-center gap-1.5">
                            <span className="text-xs text-muted-foreground">
                              知识引用
                            </span>
                            {citations.map((citation, cIdx) => (
                              <button
                                key={citation.chunkId ?? `cit_${cIdx}`}
                                type="button"
                                onClick={() =>
                                  setEvidenceDetail({
                                    kind: 'citation',
                                    data: citation,
                                  })
                                }
                                className="rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1"
                              >
                                <Badge
                                  variant="secondary"
                                  className="cursor-pointer hover:bg-control-hover"
                                >
                                  <BookOpenIcon />
                                  <span className="max-w-[160px] truncate">
                                    {citation.title || '授权知识片段'}
                                  </span>
                                  {typeof citation.sourceVersion ===
                                  'number' ? (
                                    <span className="text-muted-foreground">
                                      v{citation.sourceVersion}
                                    </span>
                                  ) : null}
                                </Badge>
                              </button>
                            ))}
                          </div>
                        ) : null}

                        {webSources.length > 0 ? (
                          <div className="flex flex-wrap items-center gap-1.5">
                            <span className="text-xs text-muted-foreground">
                              网页来源
                            </span>
                            {webSources.map((source, sIdx) => (
                              <button
                                key={source.url ?? `web_${sIdx}`}
                                type="button"
                                onClick={() =>
                                  setEvidenceDetail({
                                    kind: 'web',
                                    data: source,
                                  })
                                }
                                className="rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1"
                              >
                                <Badge
                                  variant="secondary"
                                  className="cursor-pointer hover:bg-control-hover"
                                >
                                  <GlobeIcon />
                                  <span className="max-w-[160px] truncate">
                                    {source.title ||
                                      source.url ||
                                      '实时网页来源'}
                                  </span>
                                </Badge>
                              </button>
                            ))}
                          </div>
                        ) : null}
                      </div>
                    ) : null}
                  </div>
                );
              })
            )}

            {/* Running Stage Banner + Independent Cancel Run Button */}
            {asking ? (
              <div className="flex flex-col gap-2.5 rounded-xl bg-canvas-soft p-3.5">
                <div className="flex items-center gap-2 text-sm text-foreground">
                  <Spinner className="size-4 shrink-0" />
                  <span>正在检索授权知识库并执行实时联网搜索…</span>
                </div>
                <div className="flex items-center justify-between gap-2">
                  <span className="text-xs text-muted-foreground">
                    收起面板后仍会继续运行；如需中断请点击取消。
                  </span>
                  <Button
                    type="button"
                    variant="secondary"
                    size="xs"
                    disabled={cancelling}
                    onClick={() => void handleCancelRun()}
                  >
                    <SquareIcon />
                    {cancelling ? '正在取消…' : '取消运行'}
                  </Button>
                </div>
              </div>
            ) : null}

            {/* Structured Error Alert with Actionable Recovery */}
            {lastError ? (
              <Alert variant="destructive">
                <AlertCircleIcon />
                <AlertTitle>{lastError.title}</AlertTitle>
                <AlertDescription className="space-y-2.5">
                  <p>{lastError.description}</p>
                  <div className="flex flex-wrap items-center gap-2">
                    {lastError.kind === 'auth' ? (
                      <Button size="xs" asChild>
                        <Link
                          href={buildLoginHref(pathname)}
                          onClick={() => setOpen(false)}
                        >
                          重新登录
                        </Link>
                      </Button>
                    ) : null}
                    {lastError.kind === 'verify' ? (
                      <Button size="xs" asChild>
                        <Link href="/account" onClick={() => setOpen(false)}>
                          前往验证邮箱
                        </Link>
                      </Button>
                    ) : null}
                    {lastError.retryable && inputVal.trim() ? (
                      <Button
                        type="button"
                        size="xs"
                        disabled={asking}
                        onClick={() => void handleSend()}
                      >
                        <RefreshCwIcon />
                        重新尝试
                      </Button>
                    ) : null}
                    {lastError.kind === 'revoked' ? (
                      <Button
                        type="button"
                        variant="secondary"
                        size="xs"
                        onClick={handleStartNewSession}
                      >
                        开启新对话
                      </Button>
                    ) : null}
                    <Button
                      type="button"
                      variant="ghost"
                      size="xs"
                      onClick={() => setLastError(null)}
                    >
                      知道了
                    </Button>
                  </div>
                </AlertDescription>
              </Alert>
            ) : null}

            <div ref={messagesEndRef} />
          </div>

          {/* Input Footer using official InputGroup */}
          <div className="shrink-0 bg-background px-4 pt-2 pb-4">
            <InputGroup>
              <InputGroupTextarea
                value={inputVal}
                onChange={(e) => setInputVal(e.target.value)}
                onCompositionStart={() => {
                  isComposingRef.current = true;
                }}
                onCompositionEnd={() => {
                  isComposingRef.current = false;
                }}
                onKeyDown={handleTextareaKeyDown}
                placeholder={
                  publishedAgents.length === 0
                    ? '暂无可用助手实例'
                    : summary?.paused
                      ? '平台已暂停新增 AI 请求'
                      : '输入你的问题（Enter 发送，Shift + Enter 换行）…'
                }
                disabled={
                  asking ||
                  publishedAgents.length === 0 ||
                  Boolean(summary?.paused)
                }
                aria-label="向学屿助手提问"
                rows={2}
              />
              <InputGroupAddon align="block-end">
                <InputGroupText className="text-xs">
                  <GlobeIcon className="size-3.5" />
                  <span>每答必联网</span>
                </InputGroupText>
                <div className="flex items-center gap-1.5">
                  {asking ? (
                    <InputGroupButton
                      variant="secondary"
                      size="xs"
                      disabled={cancelling}
                      onClick={() => void handleCancelRun()}
                    >
                      <SquareIcon />
                      取消运行
                    </InputGroupButton>
                  ) : (
                    <InputGroupButton
                      variant="default"
                      size="xs"
                      disabled={
                        !inputVal.trim() ||
                        publishedAgents.length === 0 ||
                        Boolean(summary?.paused)
                      }
                      onClick={() => void handleSend()}
                    >
                      <ArrowUpIcon />
                      发送
                    </InputGroupButton>
                  )}
                </div>
              </InputGroupAddon>
            </InputGroup>
          </div>
        </>
      )}
    </div>
  );

  return (
    <>
      {/* Trigger Button (Design 010 §5.4): 52px circular black button, inverse mark only */}
      {!open ? (
        <button
          ref={triggerRef}
          type="button"
          onClick={() => setOpen(true)}
          aria-label="打开学屿助手"
          aria-expanded={open}
          className="fixed right-4 bottom-4 z-50 inline-flex size-[52px] items-center justify-center rounded-full bg-primary text-primary-foreground shadow-[var(--overlay-shadow)] transition-transform duration-150 outline-none hover:scale-[1.03] hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background md:right-6 md:bottom-6"
        >
          <LearnisLogo variant="mark" tone="inverse" markSize={24} />
        </button>
      ) : null}

      {/* Desktop Floating Panel */}
      {open && !isMobile ? (
        <section
          role="dialog"
          aria-label="学屿助手"
          className="fixed right-6 bottom-6 z-50 flex h-[min(680px,calc(100vh-112px))] w-[440px] flex-col overflow-hidden rounded-2xl bg-popover text-popover-foreground shadow-[var(--overlay-shadow)]"
        >
          {renderPanelContent()}
        </section>
      ) : null}

      {/* Mobile Full-screen Official Sheet */}
      {isMobile ? (
        <Sheet open={open} onOpenChange={setOpen}>
          <SheetContent
            side="right"
            showCloseButton={false}
            className="inset-0 h-dvh w-full max-w-none gap-0 p-0 sm:max-w-none"
          >
            <SheetHeader className="sr-only">
              <SheetTitle>学屿助手</SheetTitle>
              <SheetDescription>
                结合授权知识库与实时联网搜索的学习问答浮层
              </SheetDescription>
            </SheetHeader>
            {renderPanelContent()}
          </SheetContent>
        </Sheet>
      ) : null}

      {/* Citation & Web Source Evidence Detail Dialog */}
      <Dialog
        open={Boolean(evidenceDetail)}
        onOpenChange={(nextOpen) => {
          if (!nextOpen) {
            setEvidenceDetail(null);
          }
        }}
      >
        <DialogContent className="sm:max-w-md">
          {evidenceDetail?.kind === 'citation' ? (
            <>
              <DialogHeader>
                <div className="flex items-center gap-2">
                  <Badge variant="secondary">
                    <BookOpenIcon />
                    知识库引用
                  </Badge>
                  {typeof evidenceDetail.data.sourceVersion === 'number' ? (
                    <Badge variant="secondary">
                      版本 v{evidenceDetail.data.sourceVersion}
                    </Badge>
                  ) : null}
                </div>
                <DialogTitle className="mt-1">
                  {evidenceDetail.data.title || '授权知识片段'}
                </DialogTitle>
                <DialogDescription>
                  来自已发布且经实时权限校验的知识库来源切片
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-3">
                <div className="rounded-xl bg-canvas-soft p-4 text-sm leading-6 whitespace-pre-wrap text-body">
                  {evidenceDetail.data.snippet || '暂无摘要片段。'}
                </div>
                <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                  {typeof evidenceDetail.data.chunkIndex === 'number' ? (
                    <span>切片序号：#{evidenceDetail.data.chunkIndex + 1}</span>
                  ) : null}
                  {evidenceDetail.data.courseId ? (
                    <span>关联课程：{evidenceDetail.data.courseId}</span>
                  ) : null}
                </div>
              </div>
              <DialogFooter showCloseButton />
            </>
          ) : evidenceDetail?.kind === 'web' ? (
            <>
              <DialogHeader>
                <div className="flex items-center gap-2">
                  <Badge variant="secondary">
                    <GlobeIcon />
                    实时网页来源
                  </Badge>
                </div>
                <DialogTitle className="mt-1">
                  {evidenceDetail.data.title || '网页检索结果'}
                </DialogTitle>
                <DialogDescription>
                  本次问答触发实时联网搜索获取的公开网页参考
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-3">
                <div className="rounded-xl bg-canvas-soft p-4 text-sm leading-6 whitespace-pre-wrap text-body">
                  {evidenceDetail.data.snippet || '暂无网页摘要内容。'}
                </div>
                {evidenceDetail.data.url ? (
                  <p className="truncate font-mono text-xs text-muted-foreground">
                    {evidenceDetail.data.url}
                  </p>
                ) : null}
              </div>
              <DialogFooter>
                {isSafeExternalHref(evidenceDetail.data.url) ? (
                  <Button asChild>
                    <a
                      href={evidenceDetail.data.url}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      <ExternalLinkIcon />
                      打开外部链接
                    </a>
                  </Button>
                ) : null}
                <Button
                  variant="secondary"
                  onClick={() => setEvidenceDetail(null)}
                >
                  关闭
                </Button>
              </DialogFooter>
            </>
          ) : null}
        </DialogContent>
      </Dialog>
    </>
  );
}
