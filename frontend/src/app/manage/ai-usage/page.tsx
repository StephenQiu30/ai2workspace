import {Layout} from '@/components/layout/layout';
import type {Metadata} from 'next';
import {ManageNav} from '@/components/layout/manage_nav';
import {AiUsageView} from '@/features/assistant/ai_usage_view';
import {ManageAccessGuard} from '@/features/assistant/manage_access_guard';
import {isAdmin} from '@/lib/auth/roles';
import {getServerSession} from '@/lib/auth/session';
import {getErrorMessage} from '@/lib/format';
import {getServerRequestOptions} from '@/lib/request/server';
import {
  assistantGetGovernanceSummary,
  assistantListModels,
  assistantListReconciliations,
} from '@/services/generated/api/governance';

export const metadata: Metadata = {
  title: '模型与 AI 费用治理 · ai2workspace',
  description:
    '管理员专属控制台：四家受控模型目录、平台日月预算、后台入库子额度、80% 预警与未知费用核对队列。',
};

export const dynamic = 'force-dynamic';

export default async function AiUsageManagePage() {
  const user = await getServerSession();

  if (!isAdmin(user)) {
    return (
      <Layout as="section" className="w-full">
        <ManageNav user={user} />
        <ManageAccessGuard
          user={user}
          currentPath="/manage/ai-usage"
          requiredRoleLabel="平台管理员"
          title={
            user
              ? '仅平台管理员可访问模型与 AI 费用治理'
              : '请先使用管理员账号登录'
          }
          description={
            user
              ? '当前账号不具备平台管理员权限，无法查看或修改四家模型价目目录、平台预算上限及未知费用核对队列。'
              : '该模块仅对平台管理员开放，请先登录管理员账号。'
          }
        />
      </Layout>
    );
  }

  let initialSummary: Api.UsageSummary | null = null;
  let initialModels: Api.ModelInfo[] = [];
  let initialReconciliations: Api.ReconciliationItem[] = [];
  let initialError: string | null = null;

  try {
    const options = await getServerRequestOptions();
    const [summaryRes, modelsRes, reconciliationsRes] = await Promise.all([
      assistantGetGovernanceSummary(options),
      assistantListModels(options),
      assistantListReconciliations(options),
    ]);
    initialSummary = summaryRes ?? null;
    if (Array.isArray(modelsRes)) {
      initialModels = modelsRes;
    }
    if (Array.isArray(reconciliationsRes)) {
      initialReconciliations = reconciliationsRes;
    }
  } catch (err: unknown) {
    initialError = getErrorMessage(err, '初始化模型与 AI 费用治理数据失败');
  }

  return (
    <Layout as="section" className="w-full">
      <ManageNav user={user} />
      <AiUsageView
        initialSummary={initialSummary}
        initialModels={initialModels}
        initialReconciliations={initialReconciliations}
        initialError={initialError}
      />
    </Layout>
  );
}
