import {Layout} from '@/components/layout/layout';
import type {Metadata} from 'next';
import {ManageNav} from '@/components/layout/manage_nav';
import {KnowledgeDetailView} from '@/features/assistant/knowledge_detail_view';
import {ManageAccessGuard} from '@/features/assistant/manage_access_guard';
import {canManageKnowledge} from '@/lib/auth/roles';
import {getServerSession} from '@/lib/auth/session';
import {getErrorMessage} from '@/lib/format';
import {getServerRequestOptions} from '@/lib/request/server';
import {
  assistantGetKnowledgeBase,
  assistantListKnowledgeSources,
} from '@/services/generated/assistant/knowledge';

export const metadata: Metadata = {
  title: '知识库来源与版本治理 · ai2workspace',
  description:
    '分步管理知识来源登记、内容升版、切片入库索引、就绪版本发布与即时撤回。',
};

export const dynamic = 'force-dynamic';

export interface KnowledgeDetailPageProps {
  params: Promise<{
    knowledge_base_id: string;
  }>;
}

export default async function KnowledgeDetailPage({
  params,
}: KnowledgeDetailPageProps) {
  const {knowledge_base_id: knowledgeBaseId} = await params;
  const user = await getServerSession();

  if (!canManageKnowledge(user)) {
    return (
      <Layout as="section" className="w-full">
        <ManageNav user={user} />
        <ManageAccessGuard
          user={user}
          currentPath={`/manage/knowledge/${knowledgeBaseId}`}
          requiredRoleLabel="讲师或管理员"
          title={user ? '需要讲师或管理员权限' : '请先登录后访问知识库详情'}
        />
      </Layout>
    );
  }

  let initialBase: AssistantApi.KnowledgeBase | null = null;
  let initialSources: AssistantApi.KnowledgeSource[] = [];
  let initialError: string | null = null;

  try {
    const options = await getServerRequestOptions();
    const [baseRes, sourcesRes] = await Promise.all([
      assistantGetKnowledgeBase({id: knowledgeBaseId}, options),
      assistantListKnowledgeSources({id: knowledgeBaseId}, options),
    ]);
    initialBase = baseRes;
    if (Array.isArray(sourcesRes)) {
      initialSources = sourcesRes;
    }
  } catch (err: unknown) {
    initialError = getErrorMessage(err, '加载知识库详情或来源列表失败');
  }

  return (
    <Layout as="section" className="w-full">
      <ManageNav user={user} />
      <KnowledgeDetailView
        knowledgeBaseId={knowledgeBaseId}
        initialBase={initialBase}
        initialSources={initialSources}
        initialError={initialError}
      />
    </Layout>
  );
}
