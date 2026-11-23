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
import {AccountManagement} from '@/features/account/account_management';
import {isAdmin} from '@/lib/auth/roles';
import {getServerSession} from '@/lib/auth/session';
import {buildLoginHref, getErrorMessage} from '@/lib/format';
import {getServerRequestOptions} from '@/lib/request/server';
import {
  identityListAccounts,
  identityListInstructorInvitations,
} from '@/services/generated/identity/access';

export const metadata: Metadata = {
  title: '账号管理',
  description: '管理员控制台：用户列表、账号状态处置与受控讲师邀请管理。',
};

export const dynamic = 'force-dynamic';

export default async function ManageAccountsPage() {
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
                    ? '当前账号不具备管理员角色，无法访问账号管理与讲师邀请控制台。'
                    : '请先使用管理员账号登录后再访问账号管理页面。'}
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                {user ? (
                  <Button asChild variant="secondary">
                    <Link href="/">返回社区首页</Link>
                  </Button>
                ) : (
                  <Button asChild>
                    <Link href={buildLoginHref('/manage/accounts')}>
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

  let initialAccounts: IdentityApi.ListAccountsResponse = {
    items: [],
    page: 1,
    pageSize: 20,
    total: 0,
  };
  let initialInvitations: IdentityApi.ListInvitationsResponse = {
    items: [],
    page: 1,
    pageSize: 20,
    total: 0,
  };
  let initialError: string | null = null;

  try {
    const options = await getServerRequestOptions();
    const [accountsResp, invitationsResp] = await Promise.all([
      identityListAccounts({page: 1, pageSize: 20}, options),
      identityListInstructorInvitations({page: 1, pageSize: 20}, options),
    ]);
    initialAccounts = accountsResp;
    initialInvitations = invitationsResp;
  } catch (err: unknown) {
    initialError = getErrorMessage(err, '初始化账号管理数据失败');
  }

  return (
    <Layout as="section" className="w-full">
      <ManageNav user={user} />
      <AccountManagement
        currentUser={user!}
        initialAccounts={initialAccounts}
        initialInvitations={initialInvitations}
        initialError={initialError}
      />
    </Layout>
  );
}
