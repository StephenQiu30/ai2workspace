import type {Metadata} from 'next';
import {ManageNav} from '@/components/layout/manage_nav';
import {AgentConfigView} from '@/features/assistant/agent_config_view';
import {ManageAccessGuard} from '@/features/assistant/manage_access_guard';
import {isAdmin} from '@/lib/auth/roles';
import {getServerSession} from '@/lib/auth/session';
import {getErrorMessage} from '@/lib/format';
import {getServerRequestOptions} from '@/lib/request/server';
import {
  assistantGetAgentBySlug,
  assistantListAgents,
} from '@/services/generated/assistant/agents';
import {assistantListModels} from '@/services/generated/assistant/governance';
import {assistantListKnowledgeBases} from '@/services/generated/assistant/knowledge';

export const metadata: Metadata = {
  title: 'Agent 实例详细配置 · ai2workspace',
  description:
    '管理员专属控制台：配置 Agent 角色提示词、受控模型与多选授权知识库（R-28）。',
};

export const dynamic = 'force-dynamic';

export interface AgentConfigPageProps {
  params: Promise<{
    agent_id: string;
  }>;
}

export default async function AgentConfigPage({params}: AgentConfigPageProps) {
  const {agent_id: rawAgentParam} = await params;
  const agentParam = decodeURIComponent(rawAgentParam);
  const user = await getServerSession();

  // Strictly enforce R-28: only admin can configure Agent instances.
  if (!isAdmin(user)) {
    return (
      <section className="mx-auto w-full max-w-5xl px-4 py-8 sm:px-6">
        <ManageNav user={user} />
        <ManageAccessGuard
          user={user}
          currentPath={`/manage/agents/${encodeURIComponent(agentParam)}`}
          requiredRoleLabel="平台管理员"
          title={
            user
              ? '仅平台管理员可配置 Agent 实例（R-28）'
              : '请先使用管理员账号登录'
          }
          description={
            user
              ? '根据平台治理规范（R-28），讲师与普通学员账号无权访问 Agent 配置详情页。'
              : '该模块仅对平台管理员开放，请先登录管理员账号。'
          }
        />
      </section>
    );
  }

  let initialAgent: AssistantApi.Configuration | null = null;
  let initialModels: AssistantApi.ModelInfo[] = [];
  let initialKnowledgeBases: AssistantApi.KnowledgeBase[] = [];
  let initialError: string | null = null;

  try {
    const options = await getServerRequestOptions();
    const [agentList, modelList, kbList] = await Promise.all([
      assistantListAgents(options),
      assistantListModels(options),
      assistantListKnowledgeBases(options),
    ]);

    if (Array.isArray(modelList)) {
      initialModels = modelList;
    }
    if (Array.isArray(kbList)) {
      initialKnowledgeBases = kbList;
    }

    if (Array.isArray(agentList)) {
      initialAgent =
        agentList.find(
          (item) => item.id === agentParam || item.slug === agentParam,
        ) ?? null;
    }

    if (!initialAgent) {
      const bySlug = await assistantGetAgentBySlug({slug: agentParam}, options);
      initialAgent = bySlug ?? null;
    }
  } catch (err: unknown) {
    initialError = getErrorMessage(err, '加载 Agent 配置详情失败');
  }

  return (
    <section className="mx-auto w-full max-w-6xl px-4 py-8 sm:px-6">
      <ManageNav user={user} />
      <AgentConfigView
        agentParam={agentParam}
        initialAgent={initialAgent}
        initialModels={initialModels}
        initialKnowledgeBases={initialKnowledgeBases}
        initialError={initialError}
      />
    </section>
  );
}
