'use client';

import React, {useState, useEffect, useCallback} from 'react';
import {
  getApiAssistantAgents,
  postApiAssistantAgents,
  postApiAssistantAgentsIdPublish,
} from '@/services/generated/assistant/agents';
import {getApiAssistantModels} from '@/services/generated/assistant/governance';
import {getApiAssistantKnowledgeBases} from '@/services/generated/assistant/knowledge';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Card, CardHeader, CardTitle, CardContent} from '@/components/ui/card';
import {Badge} from '@/components/ui/badge';

export default function AgentManagePage() {
  const [agents, setAgents] = useState<AssistantApi.Configuration[]>([]);
  const [models, setModels] = useState<AssistantApi.ModelInfo[]>([]);
  const [bases, setBases] = useState<AssistantApi.KnowledgeBase[]>([]);
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState('');
  const [successMsg, setSuccessMsg] = useState('');

  // Form states
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [instruction, setInstruction] = useState('');
  const [providerId, setProviderId] = useState('openai');
  const [modelId, setModelId] = useState('gpt-4o-mini');
  const [selectedKbId, setSelectedKbId] = useState('');

  const loadData = useCallback(async () => {
    try {
      const [agentList, modelList, kbList] = await Promise.all([
        getApiAssistantAgents(),
        getApiAssistantModels(),
        getApiAssistantKnowledgeBases(),
      ]);
      if (Array.isArray(agentList)) setAgents(agentList);
      if (Array.isArray(modelList)) setModels(modelList);
      if (Array.isArray(kbList)) {
        setBases(kbList);
        setSelectedKbId((prev) => (prev ? prev : (kbList[0]?.id ?? '')));
      }
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : '加载配置数据失败');
    }
  }, []);

  useEffect(() => {
    let isMounted = true;
    void (async () => {
      if (!isMounted) return;
      await loadData();
    })();
    return () => {
      isMounted = false;
    };
  }, [loadData]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !slug.trim() || !instruction.trim()) return;

    setLoading(true);
    setErrorMsg('');
    try {
      const created = await postApiAssistantAgents({
        name: name.trim(),
        slug: slug.trim(),
        roleInstruction: instruction.trim(),
        providerId,
        modelId,
        knowledgeBaseIds: selectedKbId ? [selectedKbId] : [],
      });
      if (created && created.id) {
        // Auto publish
        await postApiAssistantAgentsIdPublish({id: created.id});
        setSuccessMsg(`Agent 实例「${created.name}」已创建并发布`);
        setName('');
        setSlug('');
        setInstruction('');
        await loadData();
      }
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : '创建 Agent 失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="container mx-auto max-w-5xl py-8 px-4 space-y-8">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">
          AI Agent 实例与模型配置
        </h1>
        <p className="text-sm text-muted-foreground mt-1">
          配置并发布官方或特定课程的 Agent
          实例，绑定四家受控模型提供商与授权知识库。
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

      <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
        {/* Existing Agents */}
        <div className="space-y-4">
          <h2 className="font-semibold text-base">
            已部署 Agent 实例 ({agents.length})
          </h2>
          <div className="space-y-3">
            {agents.map((ag) => (
              <Card key={ag.id}>
                <CardHeader className="pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-base">{ag.name}</CardTitle>
                    <Badge
                      variant={
                        ag.state === 'published' ? 'default' : 'secondary'
                      }
                    >
                      {ag.state === 'published' ? '已发布' : '草稿'}
                    </Badge>
                  </div>
                  <div className="text-xs text-muted-foreground">
                    标识符: <span className="font-mono">{ag.slug}</span> ·
                    发布版本: v{ag.publishedVersion}
                  </div>
                </CardHeader>
                <CardContent className="text-xs space-y-2">
                  <p className="text-muted-foreground line-clamp-2">
                    {ag.roleInstruction}
                  </p>
                  <div className="flex items-center gap-2 pt-1 border-t border-border">
                    <Badge variant="outline" className="text-[11px]">
                      {ag.providerId} / {ag.modelId}
                    </Badge>
                    <span className="text-muted-foreground">
                      绑定知识库: {ag.knowledgeBaseIds?.length ?? 0} 个
                    </span>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>

        {/* Create Agent Form */}
        <div>
          <Card>
            <CardHeader>
              <CardTitle className="text-base">新建并发布 Agent 实例</CardTitle>
            </CardHeader>
            <CardContent>
              <form
                onSubmit={(e) => void handleCreate(e)}
                className="space-y-4"
              >
                <div className="space-y-1">
                  <label className="text-xs font-medium text-foreground">
                    实例名称
                  </label>
                  <Input
                    placeholder="如：全栈答疑专属助教"
                    value={name}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                      setName(e.target.value)
                    }
                    required
                  />
                </div>

                <div className="space-y-1">
                  <label className="text-xs font-medium text-foreground">
                    唯一标识符 (Slug)
                  </label>
                  <Input
                    placeholder="如：fullstack_tutor"
                    value={slug}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                      setSlug(e.target.value)
                    }
                    required
                  />
                </div>

                <div className="space-y-1">
                  <label className="text-xs font-medium text-foreground">
                    模型提供商与型号
                  </label>
                  <div className="grid grid-cols-2 gap-2">
                    <select
                      className="h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-xs shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
                      value={providerId}
                      onChange={(e: React.ChangeEvent<HTMLSelectElement>) =>
                        setProviderId(e.target.value)
                      }
                    >
                      <option value="deepseek">DeepSeek</option>
                      <option value="openai">OpenAI</option>
                      <option value="openrouter">OpenRouter</option>
                      <option value="volcengine">火山引擎</option>
                    </select>

                    <select
                      className="h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-xs shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
                      value={modelId}
                      onChange={(e: React.ChangeEvent<HTMLSelectElement>) =>
                        setModelId(e.target.value)
                      }
                    >
                      {models
                        .filter((m) => m.providerId === providerId)
                        .map((m) => (
                          <option key={m.modelId} value={m.modelId}>
                            {m.displayName}
                          </option>
                        ))}
                      {models.filter((m) => m.providerId === providerId)
                        .length === 0 && (
                        <option value="default-chat">默认模型</option>
                      )}
                    </select>
                  </div>
                </div>

                <div className="space-y-1">
                  <label className="text-xs font-medium text-foreground">
                    关联知识库
                  </label>
                  <select
                    className="h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-xs shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
                    value={selectedKbId}
                    onChange={(e: React.ChangeEvent<HTMLSelectElement>) =>
                      setSelectedKbId(e.target.value)
                    }
                  >
                    <option value="">不关联额外知识库</option>
                    {bases.map((kb) => (
                      <option key={kb.id} value={kb.id}>
                        {kb.name}
                      </option>
                    ))}
                  </select>
                </div>

                <div className="space-y-1">
                  <label className="text-xs font-medium text-foreground">
                    角色指导指令 (Role Instruction)
                  </label>
                  <textarea
                    className="w-full rounded-md border border-input bg-background p-3 text-xs shadow-sm focus:outline-none focus:ring-1 focus:ring-ring leading-relaxed"
                    placeholder="设定 AI 的身份、专业范围、回答风格及严格遵循的规范..."
                    value={instruction}
                    onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
                      setInstruction(e.target.value)
                    }
                    rows={4}
                    required
                  />
                </div>

                <Button type="submit" disabled={loading} className="w-full">
                  保存并立即发布
                </Button>
              </form>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
