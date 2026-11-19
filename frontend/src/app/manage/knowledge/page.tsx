'use client';

import React, {useState, useEffect, useCallback} from 'react';
import {
  getApiAssistantKnowledgeBases,
  postApiAssistantKnowledgeBases,
  getApiAssistantKnowledgeBasesIdSources,
  postApiAssistantKnowledgeBasesIdSources,
  postApiAssistantKnowledgeSourcesIdIndexJobs,
  postApiAssistantKnowledgeSourcesIdPublish,
  postApiAssistantKnowledgeSourcesIdRevoke,
} from '@/services/generated/assistant/knowledge';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Card, CardHeader, CardTitle, CardContent} from '@/components/ui/card';
import {Badge} from '@/components/ui/badge';

export default function KnowledgeManagePage() {
  const [bases, setBases] = useState<AssistantApi.KnowledgeBase[]>([]);
  const [selectedBaseId, setSelectedBaseId] = useState<string>('');
  const [sources, setSources] = useState<AssistantApi.KnowledgeSource[]>([]);
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState('');
  const [successMsg, setSuccessMsg] = useState('');

  // Form states for creating KB
  const [kbName, setKbName] = useState('');
  const [kbDesc, setKbDesc] = useState('');

  // Form states for adding source
  const [srcTitle, setSrcTitle] = useState('');
  const [srcContent, setSrcContent] = useState('');

  const loadBases = useCallback(async () => {
    try {
      const res = await getApiAssistantKnowledgeBases();
      if (Array.isArray(res)) {
        setBases(res);
        setSelectedBaseId((prev) => (prev ? prev : (res[0]?.id ?? '')));
      }
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : '加载知识库失败');
    }
  }, []);

  const loadSources = useCallback(async (kbId: string) => {
    if (!kbId) return;
    try {
      const res = await getApiAssistantKnowledgeBasesIdSources({id: kbId});
      if (Array.isArray(res)) {
        setSources(res);
      }
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : '加载知识来源失败');
    }
  }, []);

  useEffect(() => {
    let isMounted = true;
    void (async () => {
      if (!isMounted) return;
      await loadBases();
    })();
    return () => {
      isMounted = false;
    };
  }, [loadBases]);

  useEffect(() => {
    let isMounted = true;
    if (selectedBaseId) {
      void (async () => {
        if (!isMounted) return;
        await loadSources(selectedBaseId);
      })();
    }
    return () => {
      isMounted = false;
    };
  }, [selectedBaseId, loadSources]);

  const handleCreateKB = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!kbName.trim()) return;
    setLoading(true);
    setErrorMsg('');
    try {
      const newKb = await postApiAssistantKnowledgeBases({
        name: kbName.trim(),
        description: kbDesc.trim(),
        visibility: 'public',
      });
      if (newKb && newKb.id) {
        setKbName('');
        setKbDesc('');
        setSuccessMsg(`知识库「${newKb.name}」创建成功`);
        await loadBases();
        setSelectedBaseId(newKb.id);
      }
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : '创建知识库失败');
    } finally {
      setLoading(false);
    }
  };

  const handleAddSource = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedBaseId || !srcTitle.trim() || !srcContent.trim()) return;
    setLoading(true);
    setErrorMsg('');
    try {
      const src = await postApiAssistantKnowledgeBasesIdSources(
        {id: selectedBaseId},
        {
          kind: 'manual_doc',
          title: srcTitle.trim(),
          content: srcContent.trim(),
          sourceRef: 'doc_manual',
        },
      );
      if (src && src.id) {
        // Trigger index job
        await postApiAssistantKnowledgeSourcesIdIndexJobs({id: src.id});
        // Auto publish
        await postApiAssistantKnowledgeSourcesIdPublish(
          {id: src.id},
          {expectedVersion: 1},
        );
        setSrcTitle('');
        setSrcContent('');
        setSuccessMsg(`资料「${src.title}」已完成切片索引并发布`);
        await loadSources(selectedBaseId);
      }
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : '添加资料失败');
    } finally {
      setLoading(false);
    }
  };

  const handleRevoke = async (srcId: string) => {
    try {
      await postApiAssistantKnowledgeSourcesIdRevoke({id: srcId});
      setSuccessMsg('资料已撤回，将不再向用户与问答泄露任何切片');
      if (selectedBaseId) {
        await loadSources(selectedBaseId);
      }
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : '撤回资料失败');
    }
  };

  return (
    <div className="container mx-auto max-w-6xl py-8 px-4 space-y-8">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">
          知识库与授权检索管理
        </h1>
        <p className="text-sm text-muted-foreground mt-1">
          管理由讲师或管理员明确授权的资料库，支持即时切片索引、版本发布与撤回阻断。
        </p>
      </div>

      {errorMsg && (
        <div className="p-3 bg-destructive/10 text-destructive text-sm rounded-lg border border-destructive/20">
          {errorMsg}
        </div>
      )}

      {successMsg && (
        <div className="p-3 bg-green-500/10 text-green-700 dark:text-green-300 text-sm rounded-lg border border-green-500/20">
          {successMsg}
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* Left: KB List & Creation */}
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">创建新知识库</CardTitle>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(e) => void handleCreateKB(e)}
                className="space-y-3"
              >
                <Input
                  placeholder="知识库名称（如：Go 微服务架构）"
                  value={kbName}
                  onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                    setKbName(e.target.value)
                  }
                  disabled={loading}
                  required
                />
                <textarea
                  className="w-full rounded-md border border-input bg-background p-3 text-xs shadow-sm focus:outline-none focus:ring-1 focus:ring-ring leading-relaxed"
                  placeholder="简介描述..."
                  value={kbDesc}
                  onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
                    setKbDesc(e.target.value)
                  }
                  disabled={loading}
                  rows={2}
                />
                <Button
                  type="submit"
                  disabled={loading || !kbName.trim()}
                  className="w-full"
                >
                  新建知识库
                </Button>
              </form>
            </CardContent>
          </Card>

          <div className="space-y-2">
            <h3 className="font-semibold text-sm text-muted-foreground">
              已注册知识库 ({bases.length})
            </h3>
            <div className="space-y-2">
              {bases.map((kb) => (
                <div
                  key={kb.id}
                  onClick={() => setSelectedBaseId(kb.id ?? '')}
                  className={`p-3 rounded-lg border cursor-pointer transition-colors ${
                    selectedBaseId === kb.id
                      ? 'border-primary bg-primary/5'
                      : 'border-border hover:bg-muted/50'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <span className="font-medium text-sm">{kb.name}</span>
                    <Badge variant="outline" className="text-xs">
                      v{kb.version}
                    </Badge>
                  </div>
                  {kb.description && (
                    <p className="text-xs text-muted-foreground mt-1 line-clamp-1">
                      {kb.description}
                    </p>
                  )}
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Right: Sources in selected KB */}
        <div className="md:col-span-2 space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">向当前知识库录入资料</CardTitle>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(e) => void handleAddSource(e)}
                className="space-y-4"
              >
                <Input
                  placeholder="资料标题（如：第1章 并发模型与微服务设计规范）"
                  value={srcTitle}
                  onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                    setSrcTitle(e.target.value)
                  }
                  disabled={loading || !selectedBaseId}
                  required
                />
                <textarea
                  className="w-full rounded-md border border-input bg-background p-3 text-xs shadow-sm focus:outline-none focus:ring-1 focus:ring-ring leading-relaxed"
                  placeholder="资料正文内容（支持段落切片；段落间使用空行分隔）..."
                  value={srcContent}
                  onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
                    setSrcContent(e.target.value)
                  }
                  disabled={loading || !selectedBaseId}
                  rows={4}
                  required
                />
                <Button
                  type="submit"
                  disabled={loading || !selectedBaseId || !srcTitle.trim()}
                >
                  登记并立即切片发布
                </Button>
              </form>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">资料列表与切片状态</CardTitle>
            </CardHeader>
            <CardContent>
              {sources.length === 0 ? (
                <div className="text-sm text-muted-foreground py-8 text-center">
                  当前知识库暂未录入任何资料。
                </div>
              ) : (
                <div className="space-y-3">
                  {sources.map((src) => (
                    <div
                      key={src.id}
                      className="p-4 rounded-lg border border-border flex items-center justify-between gap-4"
                    >
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="font-medium text-sm">
                            {src.title}
                          </span>
                          <Badge
                            variant={
                              src.state === 'published'
                                ? 'default'
                                : 'secondary'
                            }
                            className="text-xs"
                          >
                            {src.state === 'published'
                              ? '已发布可检索'
                              : src.state === 'revoked'
                                ? '已撤回阻断'
                                : '草稿/索引中'}
                          </Badge>
                          <span className="text-xs text-muted-foreground">
                            类型:{' '}
                            {src.kind === 'course_text'
                              ? '课程图文'
                              : '独立文档'}
                          </span>
                        </div>
                        <div className="text-xs text-muted-foreground mt-1">
                          版本: v{src.sourceVersion} · 就绪版本: v
                          {src.readyVersion}
                        </div>
                      </div>

                      <div className="flex items-center gap-2">
                        {src.state === 'published' ? (
                          <Button
                            variant="destructive"
                            size="sm"
                            onClick={() => void handleRevoke(src.id ?? '')}
                          >
                            撤回
                          </Button>
                        ) : (
                          <Badge
                            variant="outline"
                            className="text-xs text-muted-foreground"
                          >
                            不可检索
                          </Badge>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
