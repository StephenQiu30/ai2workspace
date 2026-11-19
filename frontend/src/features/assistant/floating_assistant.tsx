'use client';

import React, {useState, useEffect, useRef} from 'react';
import {getApiAssistantGovernanceSummary} from '@/services/generated/assistant/governance';
import {getApiAssistantAgents} from '@/services/generated/assistant/agents';
import {
  postApiAssistantSessions,
  getApiAssistantSessionsIdMessages,
  postApiAssistantSessionsIdAsk,
} from '@/services/generated/assistant/chat';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Badge} from '@/components/ui/badge';

interface FloatingAssistantProps {
  currentCourseId?: string;
}

export function FloatingAssistant({currentCourseId}: FloatingAssistantProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [agents, setAgents] = useState<AssistantApi.Configuration[]>([]);
  const [selectedSlug, setSelectedSlug] = useState<string>('customer_support');
  const [sessionId, setSessionId] = useState<string>('');
  const [messages, setMessages] = useState<AssistantApi.Message[]>([]);
  const [inputVal, setInputVal] = useState('');
  const [loading, setLoading] = useState(false);
  const [summary, setSummary] = useState<AssistantApi.UsageSummary | null>(
    null,
  );
  const [errorMsg, setErrorMsg] = useState('');

  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Load agents and governance summary
  useEffect(() => {
    let isMounted = true;
    void (async () => {
      try {
        const [agentsRes, summaryRes] = await Promise.all([
          getApiAssistantAgents(),
          getApiAssistantGovernanceSummary(),
        ]);
        if (!isMounted) return;
        if (Array.isArray(agentsRes)) {
          setAgents(agentsRes);
        }
        if (summaryRes) {
          setSummary(summaryRes);
        }
      } catch {
        // ignore initial silent fail
      }
    })();
    return () => {
      isMounted = false;
    };
  }, []);

  // When selected slug or courseId changes, initialize a session
  useEffect(() => {
    if (!isOpen) return;
    let isMounted = true;
    void (async () => {
      try {
        setErrorMsg('');
        const sess = await postApiAssistantSessions({
          agentSlug: selectedSlug,
          courseId: currentCourseId ?? '',
          title: selectedSlug === 'course_tutor' ? '课程答疑' : '官方客服咨询',
        });
        if (!isMounted) return;
        if (sess && sess.id) {
          setSessionId(sess.id);
          const history = await getApiAssistantSessionsIdMessages({
            id: sess.id,
          });
          if (isMounted && Array.isArray(history)) {
            setMessages(history);
          }
        }
      } catch (err: unknown) {
        if (!isMounted) return;
        const msg = err instanceof Error ? err.message : '初始化会话失败';
        setErrorMsg(msg);
      }
    })();
    return () => {
      isMounted = false;
    };
  }, [isOpen, selectedSlug, currentCourseId]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({behavior: 'smooth'});
  }, [messages, loading]);

  const handleSend = async () => {
    const text = inputVal.trim();
    if (!text || loading || !sessionId) return;

    setInputVal('');
    setLoading(true);
    setErrorMsg('');

    // Optimistically add user message
    const tempUserMsg: AssistantApi.Message = {
      id: `temp_${Date.now()}`,
      sender: 'user',
      content: text,
      createdAt: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, tempUserMsg]);

    try {
      const resp = await postApiAssistantSessionsIdAsk(
        {id: sessionId},
        {question: text},
      );
      if (resp) {
        setMessages((prev) => [...prev, resp]);
      }
      // Refresh summary
      const updatedSummary = await getApiAssistantGovernanceSummary();
      if (updatedSummary) {
        setSummary(updatedSummary);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '提问失败，请稍后重试';
      setErrorMsg(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed bottom-6 right-6 z-50">
      {!isOpen ? (
        <button
          onClick={() => setIsOpen(true)}
          className="flex items-center gap-2 px-4 py-3 bg-primary text-primary-foreground rounded-full shadow-lg hover:shadow-xl transition-all duration-200 hover:scale-105 focus:outline-none"
          title="开启学屿 AI 助教"
          aria-label="开启学屿 AI 助教"
        >
          <div className="w-6 h-6 rounded-full bg-white/20 flex items-center justify-center font-bold text-xs">
            屿
          </div>
          <span className="font-medium text-sm">AI 助教</span>
          {summary && (
            <Badge
              variant="secondary"
              className="text-xs bg-white/20 hover:bg-white/30 text-white border-0"
            >
              余{' '}
              {Math.max(
                0,
                (summary.maxDailyQuestions ?? 20) -
                  (summary.questionsCount ?? 0),
              )}
            </Badge>
          )}
        </button>
      ) : (
        <div className="w-[400px] h-[580px] bg-card text-card-foreground border border-border rounded-2xl shadow-2xl flex flex-col overflow-hidden animate-in fade-in-50 zoom-in-95">
          {/* Header */}
          <div className="px-4 py-3 bg-muted/60 border-b border-border flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="w-7 h-7 rounded-full bg-primary flex items-center justify-center text-primary-foreground font-bold text-xs">
                屿
              </div>
              <div>
                <div className="font-semibold text-sm">学屿智能助教</div>
                <div className="text-[11px] text-muted-foreground">
                  已启用真实联网搜索与授权知识检索
                </div>
              </div>
            </div>
            <button
              onClick={() => setIsOpen(false)}
              className="text-muted-foreground hover:text-foreground text-sm font-semibold p-1"
              aria-label="关闭"
            >
              ✕
            </button>
          </div>

          {/* Mode Switcher & Budget Info */}
          <div className="px-4 py-2 bg-muted/20 border-b border-border flex items-center justify-between text-xs">
            <div className="flex items-center gap-1 overflow-x-auto max-w-[240px]">
              {agents.length > 0 ? (
                agents.map((ag) => (
                  <Button
                    key={ag.slug}
                    variant={selectedSlug === ag.slug ? 'default' : 'ghost'}
                    size="sm"
                    className="h-7 text-xs px-2.5 whitespace-nowrap"
                    onClick={() => setSelectedSlug(ag.slug ?? '')}
                  >
                    {ag.slug === 'customer_support'
                      ? '官方客服'
                      : ag.slug === 'course_tutor'
                        ? '课程答疑'
                        : ag.name}
                  </Button>
                ))
              ) : (
                <>
                  <Button
                    variant={
                      selectedSlug === 'customer_support' ? 'default' : 'ghost'
                    }
                    size="sm"
                    className="h-7 text-xs px-2.5"
                    onClick={() => setSelectedSlug('customer_support')}
                  >
                    官方客服
                  </Button>
                  <Button
                    variant={
                      selectedSlug === 'course_tutor' ? 'default' : 'ghost'
                    }
                    size="sm"
                    className="h-7 text-xs px-2.5"
                    onClick={() => setSelectedSlug('course_tutor')}
                  >
                    课程答疑
                  </Button>
                </>
              )}
            </div>
            {summary && (
              <span className="text-muted-foreground text-[11px]">
                今日额度: {summary.questionsCount ?? 0}/
                {summary.maxDailyQuestions ?? 20} 次
              </span>
            )}
          </div>

          {/* Messages Container */}
          <div className="flex-1 overflow-y-auto p-4 space-y-4 text-sm">
            {messages.length === 0 && (
              <div className="text-center py-12 text-muted-foreground">
                <div className="text-2xl mb-2">👋</div>
                <div className="font-medium text-foreground mb-1">
                  你好！我是你的
                  {selectedSlug === 'course_tutor'
                    ? '课程答疑助教'
                    : '学屿服务助手'}
                </div>
                <p className="text-xs max-w-[260px] mx-auto">
                  随时向我提问课程重点、技术难点或平台使用规则。每次问答均经过真实联网与知识库校验。
                </p>
              </div>
            )}

            {messages.map((m) => {
              const isUser = m.sender === 'user';
              return (
                <div
                  key={m.id}
                  className={`flex flex-col ${isUser ? 'items-end' : 'items-start'}`}
                >
                  <div
                    className={`max-w-[85%] rounded-2xl px-3.5 py-2.5 leading-relaxed whitespace-pre-wrap ${
                      isUser
                        ? 'bg-primary text-primary-foreground'
                        : 'bg-muted text-foreground'
                    }`}
                  >
                    {m.content}
                  </div>

                  {/* Assistant Citations & Search Evidence */}
                  {!isUser && (
                    <div className="mt-1.5 space-y-1 max-w-[90%] text-[11px]">
                      {m.searchQueries && m.searchQueries.length > 0 && (
                        <div className="flex items-center gap-1 text-muted-foreground">
                          <span>🌐 真实网络证据:</span>
                          <span className="font-medium truncate">
                            {m.searchQueries
                              .map(
                                (s: AssistantApi.SearchResultItem) =>
                                  s.title ?? '',
                              )
                              .join('；')}
                          </span>
                        </div>
                      )}
                      {m.citations && m.citations.length > 0 && (
                        <div className="flex flex-wrap gap-1 mt-1">
                          <span className="text-muted-foreground">
                            📚 知识引用:
                          </span>
                          {m.citations.map(
                            (c: AssistantApi.Citation, idx: number) => (
                              <Badge
                                key={c.chunkId ?? idx}
                                variant="outline"
                                className="text-[10px] py-0 px-1.5 bg-background"
                                title={c.snippet}
                              >
                                {c.title}
                              </Badge>
                            ),
                          )}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })}

            {loading && (
              <div className="flex items-center gap-2 text-xs text-muted-foreground py-2">
                <div className="animate-spin rounded-full h-3.5 w-3.5 border-2 border-primary border-t-transparent" />
                <span>正在执行联网搜索与知识推理...</span>
              </div>
            )}

            {errorMsg && (
              <div className="p-2.5 bg-destructive/10 text-destructive text-xs rounded-lg border border-destructive/20">
                {errorMsg}
              </div>
            )}
            <div ref={messagesEndRef} />
          </div>

          {/* Input Footer */}
          <div className="p-3 bg-muted/30 border-t border-border flex items-center gap-2">
            <Input
              value={inputVal}
              onChange={(e) => setInputVal(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault();
                  void handleSend();
                }
              }}
              placeholder="输入你的问题（按回车发送）..."
              className="h-9 text-xs"
              disabled={loading}
            />
            <Button
              size="sm"
              className="h-9 px-4 text-xs font-medium"
              onClick={() => void handleSend()}
              disabled={loading || !inputVal.trim()}
            >
              发送
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
