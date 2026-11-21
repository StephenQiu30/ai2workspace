import type {Metadata} from 'next';
import {ManageNav} from '@/components/layout/manage_nav';
import {AgentManageList} from '@/features/assistant/agent_manage_list';
import {ManageAccessGuard} from '@/features/assistant/manage_access_guard';
import {isAdmin} from '@/lib/auth/roles';
import {getServerSession} from '@/lib/auth/session';
import {getErrorMessage} from '@/lib/format';
import {getServerRequestOptions} from '@/lib/request/server';
import {assistantListAgents} from '@/services/generated/assistant/agents';
import {assistantListModels} from '@/services/generated/assistant/governance';
import {assistantListKnowledgeBases} from '@/services/generated/assistant/knowledge';

export const metadata: Metadata = {
  title: 'Agent 实例管理 · ai2workspace',
  description:
    '管理员专属控制台：配置、版本发布与停用学屿助手 Agent 实例（R-28）。',
};

export const dynamic = 'force-dynamic';

export default async function AgentManagePage() {
  const user = await getServerSession();

  // Strictly enforce R-28: only admin can configure Agent instances; instructors are explicitly denied.
  if (!isAdmin(user)) {
    return (
      <section className="mx-auto w-full max-w-5xl px-4 py-8 sm:px-6">
        <ManageNav user={user} />
        <ManageAccessGuard
          user={user}
          currentPath="/manage/agents"
          requiredRoleLabel="平台管理员"
          title={
            user
              ? '仅平台管理员可访问 Agent 管理（R-28）'
              : '请先使用管理员账号登录'
          }
          description={
            user
              ? '根据平台治理规范（R-28），讲师与普通学员账号无权配置、发布或停用 Agent 实例。如需管理课程资料，请前往「知识库管理」。'
              : '该模块仅对平台管理员开放，请先登录管理员账号。'
          }
        />
      </section>
    );
  }

  let initialAgents: AssistantApi.Configuration[] = [];
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
    if (Array.isArray(agentList)) {
      initialAgents = agentList;
    }
    if (Array.isArray(modelList)) {
      initialModels = modelList;
    }
    if (Array.isArray(kbList)) {
      initialKnowledgeBases = kbList;
    }
  } catch (err: unknown) {
    initialError = getErrorMessage(err, '初始化 Agent 管理数据失败');
  }

  return (
    <section className="mx-auto w-full max-w-6xl px-4 py-8 sm:px-6">
      <ManageNav user={user} />
      <AgentManageList
        initialAgents={initialAgents}
        initialModels={initialModels}
        initialKnowledgeBases={initialKnowledgeBases}
        initialError={initialError}
      />
    </section>
  );
}
