import {Layout} from '@/components/layout/layout';
import type {Metadata} from 'next';
import Link from 'next/link';
import {ShieldAlertIcon} from 'lucide-react';
import {ManageNav} from '@/components/layout/manage_nav';
import {Button} from '@/components/ui/button';
import {Card, CardContent} from '@/components/ui/card';
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import {ReportsManagement} from '@/features/community/reports_management';
import {isAdmin} from '@/lib/auth/roles';
import {getServerSession} from '@/lib/auth/session';
import {buildLoginHref, getErrorMessage} from '@/lib/format';
import {getServerRequestOptions} from '@/lib/request/server';
import {communityListReports} from '@/services/generated/api/community';

export const metadata: Metadata = {
  title: '内容举报治理',
  description:
    '管理员控制台：审核社区讨论帖与评论回复的违规举报并执行隐藏或驳回处置。',
};

export const dynamic = 'force-dynamic';

export default async function ManageReportsPage() {
  const user = await getServerSession();

  if (!isAdmin(user)) {
    return (
      <Layout as="section" className="w-full">
        <ManageNav user={user} />
        <Card>
          <CardContent className="py-12">
            <Empty>
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <ShieldAlertIcon />
                </EmptyMedia>
                <EmptyTitle>需要管理员权限</EmptyTitle>
                <EmptyDescription>
                  {user
                    ? '当前账号不具备管理员角色，无法访问内容举报治理控制台。'
                    : '请先使用管理员账号登录后再访问内容举报治理页面。'}
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                {user ? (
                  <Button asChild variant="secondary">
                    <Link href="/community">返回社区首页</Link>
                  </Button>
                ) : (
                  <Button asChild>
                    <Link href={buildLoginHref('/manage/reports')}>
                      前往登录
                    </Link>
                  </Button>
                )}
              </EmptyContent>
            </Empty>
          </CardContent>
        </Card>
      </Layout>
    );
  }

  let initialReports: Api.Report[] = [];
  let initialTotal = 0;
  let initialError: string | null = null;

  try {
    const options = await getServerRequestOptions();
    const resp = await communityListReports({page: 1, pageSize: 50}, options);
    initialReports = resp.items ?? [];
    initialTotal = resp.total ?? initialReports.length;
  } catch (err: unknown) {
    initialError = getErrorMessage(err, '初始化内容举报列表失败');
  }

  return (
    <Layout as="section" className="w-full">
      <ManageNav user={user} />
      <ReportsManagement
        initialReports={initialReports}
        initialTotal={initialTotal}
        initialError={initialError}
      />
    </Layout>
  );
}
