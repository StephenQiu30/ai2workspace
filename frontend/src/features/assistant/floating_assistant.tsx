'use client';

import {Layout} from '@/components/layout/layout';
import {Heading, Text} from '@/components/layout/typography';
import {Card} from '@/components/ui/card';
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
import {Bubble, BubbleContent} from '@/components/ui/bubble';
import {Marker, MarkerContent, MarkerIcon} from '@/components/ui/marker';
import {Message, MessageContent} from '@/components/ui/message';
import {
  MessageScroller,
  MessageScrollerButton,
  MessageScrollerProvider,
  MessageScrollerContent,
  MessageScrollerItem,
  MessageScrollerViewport,
} from '@/components/ui/message-scroller';
import {ToggleGroup, ToggleGroupItem} from '@/components/ui/toggle-group';
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
  SheetTrigger,
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

function isSafeExternalHref(url?: string): url is string {
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

  const abortControllerRef = useRef<AbortController | null>(null);
  const isComposingRef = useRef(false);
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
    <Layout
      flow="stack"
      className="h-full min-h-0 bg-popover text-popover-foreground"
    >
      {/* Panel Header */}
      <Layout
        flow="row"
        className="shrink-0 items-center justify-between gap-2 px-4 pt-4 pb-3"
      >
        <Layout flow="row" className="min-w-0 items-center gap-2.5">
          <LearnisLogo variant="mark" tone="ink" markSize={20} />
          <Layout className="min-w-0">
            <Layout flow="row" className="items-center gap-2">
              <Heading
                level={2}
                className="truncate text-base leading-6 font-medium text-foreground"
              >
                学屿助手
              </Heading>
              {courseContext ? (
                <Badge
                  variant="secondary"
                  className="max-w-[160px] gap-1"
                  title={courseContext.courseTitle ?? courseContext.courseId}
                >
                  <BookOpenIcon className="size-3 shrink-0" />
                  <Text className="truncate">
                    {courseContext.courseTitle || '当前课程'}
                  </Text>
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    type="button"
                    onClick={() => setCourseContext(null)}
                    aria-label="清除当前课程上下文"
                    className="ml-0.5"
                  >
                    <XIcon className="size-3" />
                  </Button>
                </Badge>
              ) : null}
            </Layout>
            <Text as="p" className="text-xs text-muted-foreground">
              关闭仅收起面板，不取消运行
            </Text>
          </Layout>
        </Layout>

        <Layout flow="row" className="shrink-0 items-center gap-1">
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
                    <Text>开启新对话</Text>
                  </DropdownMenuItem>
                </DropdownMenuGroup>
                <DropdownMenuSeparator />
                {sessions.length === 0 ? (
                  <Layout className="px-2.5 py-3 text-xs text-muted-foreground">
                    暂无历史对话记录
                  </Layout>
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
                          <Text className="w-full truncate text-sm">
                            {getSessionLabel(session)}
                          </Text>
                          <Text className="text-xs text-muted-foreground">
                            {formatDateTime(
                              session.updatedAt ?? session.createdAt,
                            )}
                          </Text>
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
        </Layout>
      </Layout>

      {/* Auth Gate: Guest */}
      {!user ? (
        <Layout flow="stack" className="flex-1 justify-center px-6 py-8">
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
        </Layout>
      ) : !user.emailVerified ? (
        /* Auth Gate: Unverified Email */
        <Layout flow="stack" className="flex-1 justify-center gap-4 px-6 py-8">
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
        </Layout>
      ) : (
        /* Verified User Q&A Body */
        <>
          {/* Agent Switcher & Quota Strip */}
          <Layout
            flow="row"
            className="shrink-0 flex-wrap items-center justify-between gap-2 bg-canvas-soft px-4 py-2.5"
          >
            <Layout
              flow="row"
              className="min-w-0 flex-1 items-center gap-1.5 overflow-x-auto"
            >
              {bootstrapping ? (
                <Layout
                  flow="row"
                  className="items-center gap-2 text-xs text-muted-foreground"
                >
                  <Spinner className="size-3.5" />
                  <Text>正在加载助手实例…</Text>
                </Layout>
              ) : publishedAgents.length > 0 ? (
                <ToggleGroup
                  type="single"
                  variant="filter"
                  size="sm"
                  value={selectedSlug}
                  disabled={asking}
                  aria-label="选择助手"
                  onValueChange={(slug) => {
                    if (slug) handleSwitchAgent(slug);
                  }}
                >
                  {publishedAgents
                    .filter((agent) => agent.slug)
                    .map((agent) => (
                      <ToggleGroupItem
                        key={agent.id ?? agent.slug}
                        value={agent.slug!}
                      >
                        {agent.name || agent.slug}
                      </ToggleGroupItem>
                    ))}
                </ToggleGroup>
              ) : (
                <Text className="text-xs text-muted-foreground">
                  暂无已发布的助手实例
                </Text>
              )}
            </Layout>

            {summary ? (
              <Text className="shrink-0 text-xs text-muted-foreground">
                {remainingQuestions !== null &&
                typeof summary.maxDailyQuestions === 'number'
                  ? `今日剩余 ${remainingQuestions}/${summary.maxDailyQuestions} 次`
                  : `今日已用 ${summary.questionsCount ?? 0} 次`}
              </Text>
            ) : null}
          </Layout>

          {/* Platform Paused Alert */}
          {summary?.paused ? (
            <Layout className="shrink-0 px-4 pt-3">
              <Alert variant="destructive">
                <ShieldAlertIcon />
                <AlertTitle>平台已暂停新增 AI 请求</AlertTitle>
                <AlertDescription>
                  {summary.pauseReason?.trim() ||
                    '管理员已临时开启保护暂停，当前仅可浏览历史对话。'}
                </AlertDescription>
              </Alert>
            </Layout>
          ) : null}

          {/* Messages Viewport */}
          <MessageScrollerProvider key={activeSessionId} autoScroll>
            <MessageScroller className="min-h-0 flex-1">
              <MessageScrollerViewport aria-label="对话记录">
                <MessageScrollerContent className="gap-4 px-4 py-4">
                  {!bootstrapping && publishedAgents.length === 0 ? (
                    <MessageScrollerItem messageId="unavailable">
                      <Empty className="py-8">
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
                    </MessageScrollerItem>
                  ) : loadingMessages ? (
                    <MessageScrollerItem messageId="loading">
                      <Marker className="justify-center py-12">
                        <MarkerIcon>
                          <Spinner />
                        </MarkerIcon>
                        <MarkerContent>正在加载对话…</MarkerContent>
                      </Marker>
                    </MessageScrollerItem>
                  ) : messages.length === 0 ? (
                    <MessageScrollerItem messageId="ready">
                      <Empty className="py-10">
                        <EmptyHeader>
                          <EmptyMedia variant="icon">
                            <LearnisLogo
                              variant="mark"
                              tone="ink"
                              markSize={20}
                            />
                          </EmptyMedia>
                          <EmptyTitle>
                            {activeAgent?.name || '学屿助手'}已就绪
                          </EmptyTitle>
                          <EmptyDescription>
                            可以提出课程或创作问题。
                          </EmptyDescription>
                        </EmptyHeader>
                      </Empty>
                    </MessageScrollerItem>
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
                        <MessageScrollerItem
                          key={message.id ?? `msg_${idx}`}
                          messageId={message.id ?? `msg_${idx}`}
                          scrollAnchor={isUser}
                        >
                          <Message align={isUser ? 'end' : 'start'}>
                            <MessageContent>
                              <Bubble
                                variant={isUser ? 'default' : 'muted'}
                                align={isUser ? 'end' : 'start'}
                              >
                                <BubbleContent className="whitespace-pre-wrap break-words">
                                  {message.content}
                                </BubbleContent>
                              </Bubble>

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
                              (citations.length > 0 ||
                                webSources.length > 0) ? (
                                <Layout
                                  flow="stack"
                                  className="max-w-[94%] gap-2"
                                >
                                  {citations.length > 0 ? (
                                    <Layout
                                      flow="row"
                                      className="flex-wrap items-center gap-1.5"
                                    >
                                      <Text className="text-xs text-muted-foreground">
                                        知识引用
                                      </Text>
                                      {citations.map((citation, cIdx) => (
                                        <Button
                                          variant="secondary"
                                          size="sm"
                                          key={
                                            citation.chunkId ?? `cit_${cIdx}`
                                          }
                                          type="button"
                                          onClick={() =>
                                            setEvidenceDetail({
                                              kind: 'citation',
                                              data: citation,
                                            })
                                          }
                                        >
                                          <BookOpenIcon />
                                          <Text className="max-w-[160px] truncate">
                                            {citation.title || '授权知识片段'}
                                          </Text>
                                          {typeof citation.sourceVersion ===
                                          'number' ? (
                                            <Text className="text-muted-foreground">
                                              v{citation.sourceVersion}
                                            </Text>
                                          ) : null}
                                        </Button>
                                      ))}
                                    </Layout>
                                  ) : null}

                                  {webSources.length > 0 ? (
                                    <Layout
                                      flow="row"
                                      className="flex-wrap items-center gap-1.5"
                                    >
                                      <Text className="text-xs text-muted-foreground">
                                        网页来源
                                      </Text>
                                      {webSources.map((source, sIdx) => (
                                        <Button
                                          variant="secondary"
                                          size="sm"
                                          key={source.url ?? `web_${sIdx}`}
                                          type="button"
                                          onClick={() =>
                                            setEvidenceDetail({
                                              kind: 'web',
                                              data: source,
                                            })
                                          }
                                        >
                                          <GlobeIcon />
                                          <Text className="max-w-[160px] truncate">
                                            {source.title ||
                                              source.url ||
                                              '实时网页来源'}
                                          </Text>
                                        </Button>
                                      ))}
                                    </Layout>
                                  ) : null}
                                </Layout>
                              ) : null}
                            </MessageContent>
                          </Message>
                        </MessageScrollerItem>
                      );
                    })
                  )}

                  {/* Running Stage Banner + Independent Cancel Run Button */}
                  {asking ? (
                    <MessageScrollerItem messageId="running">
                      <Layout flow="stack" className="gap-2.5">
                        <Layout
                          flow="row"
                          className="items-center gap-2 text-sm text-foreground"
                        >
                          <Spinner className="size-4 shrink-0" />
                          <Text>正在检索授权知识库并执行实时联网搜索…</Text>
                        </Layout>
                        <Layout
                          flow="row"
                          className="items-center justify-between gap-2"
                        >
                          <Text className="text-xs text-muted-foreground">
                            收起面板后仍会继续运行；如需中断请点击取消。
                          </Text>
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
                        </Layout>
                      </Layout>
                    </MessageScrollerItem>
                  ) : null}

                  {/* Structured Error Alert with Actionable Recovery */}
                  {lastError ? (
                    <MessageScrollerItem messageId="error">
                      <Alert variant="destructive">
                        <AlertCircleIcon />
                        <AlertTitle>{lastError.title}</AlertTitle>
                        <AlertDescription className="space-y-2.5">
                          <Text as="p">{lastError.description}</Text>
                          <Layout
                            flow="row"
                            className="flex-wrap items-center gap-2"
                          >
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
                                <Link
                                  href="/account"
                                  onClick={() => setOpen(false)}
                                >
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
                          </Layout>
                        </AlertDescription>
                      </Alert>
                    </MessageScrollerItem>
                  ) : null}
                </MessageScrollerContent>
              </MessageScrollerViewport>
              <MessageScrollerButton />
            </MessageScroller>
          </MessageScrollerProvider>

          {/* Input Footer using official InputGroup */}
          <Layout className="shrink-0 bg-background px-4 pt-2 pb-4">
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
                  <Text>每答必联网</Text>
                </InputGroupText>
                <Layout flow="row" className="items-center gap-1.5">
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
                </Layout>
              </InputGroupAddon>
            </InputGroup>
          </Layout>
        </>
      )}
    </Layout>
  );

  return (
    <>
      <Sheet open={open} onOpenChange={setOpen} modal={isMobile}>
        <SheetTrigger asChild>
          <Button
            size="icon-lg"
            aria-label="打开学屿助手"
            className={cn(
              'fixed right-4 bottom-4 z-40 size-[52px] md:right-6 md:bottom-6',
              open && 'invisible',
            )}
          >
            <LearnisLogo variant="mark" tone="inverse" markSize={24} />
          </Button>
        </SheetTrigger>
        <SheetContent
          side="right"
          showCloseButton={false}
          onInteractOutside={(event) => {
            if (!isMobile) event.preventDefault();
          }}
          className={cn(
            'gap-0 overflow-hidden p-0',
            isMobile
              ? 'data-[side=right]:h-dvh data-[side=right]:sm:max-w-none'
              : 'rounded-xl data-[side=right]:inset-y-auto data-[side=right]:right-6 data-[side=right]:bottom-6 data-[side=right]:h-[min(680px,calc(100dvh-112px))] data-[side=right]:w-[440px]',
          )}
        >
          <SheetHeader className="sr-only">
            <SheetTitle>学屿助手</SheetTitle>
            <SheetDescription>课程与学习问答</SheetDescription>
          </SheetHeader>
          {renderPanelContent()}
        </SheetContent>
      </Sheet>

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
                <Layout flow="row" className="items-center gap-2">
                  <Badge variant="secondary">
                    <BookOpenIcon />
                    知识库引用
                  </Badge>
                  {typeof evidenceDetail.data.sourceVersion === 'number' ? (
                    <Badge variant="secondary">
                      版本 v{evidenceDetail.data.sourceVersion}
                    </Badge>
                  ) : null}
                </Layout>
                <DialogTitle className="mt-1">
                  {evidenceDetail.data.title || '授权知识片段'}
                </DialogTitle>
                <DialogDescription>
                  来自已发布且经实时权限校验的知识库来源切片
                </DialogDescription>
              </DialogHeader>
              <Layout className="space-y-3">
                <Card
                  surface="muted"
                  className="p-4 text-sm leading-6 whitespace-pre-wrap text-body block"
                >
                  {evidenceDetail.data.snippet || '暂无摘要片段。'}
                </Card>
                <Layout
                  flow="row"
                  className="flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground"
                >
                  {typeof evidenceDetail.data.chunkIndex === 'number' ? (
                    <Text>切片序号：#{evidenceDetail.data.chunkIndex + 1}</Text>
                  ) : null}
                  {evidenceDetail.data.courseId ? (
                    <Text>关联课程：{evidenceDetail.data.courseId}</Text>
                  ) : null}
                </Layout>
              </Layout>
              <DialogFooter showCloseButton />
            </>
          ) : evidenceDetail?.kind === 'web' ? (
            <>
              <DialogHeader>
                <Layout flow="row" className="items-center gap-2">
                  <Badge variant="secondary">
                    <GlobeIcon />
                    实时网页来源
                  </Badge>
                </Layout>
                <DialogTitle className="mt-1">
                  {evidenceDetail.data.title || '网页检索结果'}
                </DialogTitle>
                <DialogDescription>
                  本次问答触发实时联网搜索获取的公开网页参考
                </DialogDescription>
              </DialogHeader>
              <Layout className="space-y-3">
                <Card
                  surface="muted"
                  className="p-4 text-sm leading-6 whitespace-pre-wrap text-body block"
                >
                  {evidenceDetail.data.snippet || '暂无网页摘要内容。'}
                </Card>
                {evidenceDetail.data.url ? (
                  <Text
                    as="p"
                    className="truncate font-mono text-xs text-muted-foreground"
                  >
                    {evidenceDetail.data.url}
                  </Text>
                ) : null}
              </Layout>
              <DialogFooter>
                {isSafeExternalHref(evidenceDetail.data.url) ? (
                  <Button asChild>
                    <Link
                      href={evidenceDetail.data.url}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      <ExternalLinkIcon />
                      打开外部链接
                    </Link>
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
