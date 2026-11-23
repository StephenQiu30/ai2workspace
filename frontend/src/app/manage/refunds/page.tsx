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
import {RefundsManagement} from '@/features/commerce/refunds_management';
import {isAdmin} from '@/lib/auth/roles';
import {getServerSession} from '@/lib/auth/session';
import {buildLoginHref, getErrorMessage} from '@/lib/format';
import {getServerRequestOptions} from '@/lib/request/server';
import {commerceListRefunds} from '@/services/generated/commerce/commerce';

export const metadata: Metadata = {
  title: '退款处理',
  description:
    '管理员控制台：课程退款申请审核、7×24h 窗口核查与渠道原路退款结果登记。',
};

export const dynamic = 'force-dynamic';

export default async function ManageRefundsPage() {
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
                    ? '当前账号不具备管理员角色，无法访问退款审核与渠道结果登记控制台。'
                    : '请先使用管理员账号登录后再访问退款处理页面。'}
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                {user ? (
                  <Button asChild variant="secondary">
                    <Link href="/">返回社区首页</Link>
                  </Button>
                ) : (
                  <Button asChild>
                    <Link href={buildLoginHref('/manage/refunds')}>
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

  let initialRefunds: CommerceApi.RefundListResponse = {
    items: [],
    page: 1,
    limit: 50,
    total: 0,
  };
  let initialError: string | null = null;

  try {
    const options = await getServerRequestOptions();
    initialRefunds = await commerceListRefunds({page: 1, limit: 50}, options);
  } catch (err: unknown) {
    initialError = getErrorMessage(err, '初始化退款列表数据失败');
  }

  return (
    <Layout as="section" className="w-full">
      <ManageNav user={user} />
      <RefundsManagement
        initialRefunds={initialRefunds}
        initialError={initialError}
      />
    </Layout>
  );
}
