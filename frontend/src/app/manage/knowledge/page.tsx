import {Layout} from '@/components/layout/layout';
import type {Metadata} from 'next';
import {ManageNav} from '@/components/layout/manage_nav';
import {KnowledgeManageList} from '@/features/assistant/knowledge_manage_list';
import {ManageAccessGuard} from '@/features/assistant/manage_access_guard';
import {canManageKnowledge} from '@/lib/auth/roles';
import {getServerSession} from '@/lib/auth/session';
import {getErrorMessage} from '@/lib/format';
import {getServerRequestOptions} from '@/lib/request/server';
import {assistantListKnowledgeBases} from '@/services/generated/api/knowledge';

export const metadata: Metadata = {
  title: '知识库管理 · ai2workspace',
  description: '管理授权知识库、可见范围与版本化知识来源。',
};

export const dynamic = 'force-dynamic';

export default async function KnowledgeManagePage() {
  const user = await getServerSession();

  if (!canManageKnowledge(user)) {
    return (
      <Layout as="section" className="w-full">
        <ManageNav user={user} />
        <ManageAccessGuard
          user={user}
          currentPath="/manage/knowledge"
          requiredRoleLabel="讲师或管理员"
          title={user ? '需要讲师或管理员权限' : '请先登录后访问知识库管理'}
        />
      </Layout>
    );
  }

  let initialBases: Api.KnowledgeBase[] = [];
  let initialError: string | null = null;

  try {
    const options = await getServerRequestOptions();
    const list = await assistantListKnowledgeBases(options);
    if (Array.isArray(list)) {
      initialBases = list;
    }
  } catch (err: unknown) {
    initialError = getErrorMessage(err, '初始化知识库列表失败');
  }

  return (
    <Layout as="section" className="w-full">
      <ManageNav user={user} />
      <KnowledgeManageList
        initialBases={initialBases}
        initialError={initialError}
      />
    </Layout>
  );
}
