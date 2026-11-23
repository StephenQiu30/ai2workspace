import {Layout} from '@/components/layout/layout';
import {Heading, Text} from '@/components/layout/typography';
import type {Metadata} from 'next';
import Link from 'next/link';
import {LockIcon} from 'lucide-react';
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
import {NotificationsView} from '@/features/community/notifications_view';
import {getServerSession} from '@/lib/auth/session';
import {buildLoginHref, getErrorMessage} from '@/lib/format';
import {getServerRequestOptions} from '@/lib/request/server';
import {communityListMyNotifications} from '@/services/generated/community/community';

export const metadata: Metadata = {
  title: '站内通知',
  description: '接收关于课程订单、退款处理、社区讨论回复与内容治理的站内提醒。',
};

export const dynamic = 'force-dynamic';

export default async function NotificationsPage() {
  const user = await getServerSession();
  const loginHref = buildLoginHref('/notifications');
  const registerHref = `${loginHref}${loginHref.includes('?') ? '&' : '?'}mode=register`;

  if (!user) {
    return (
      <Layout flow="stack" className="w-full gap-6">
        <Layout as="header" flow="stack" className="gap-1.5">
          <Heading
            level={1}
            size="page"
            className="font-heading text-foreground"
          >
            站内通知
          </Heading>
          <Text as="p" className="text-sm leading-6 text-muted-foreground">
            查阅社区互动回复、课程购买与退款进度以及内容治理通知。
          </Text>
        </Layout>

        <Card>
          <CardContent className="py-14">
            <Empty>
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <LockIcon />
                </EmptyMedia>
                <EmptyTitle>请先登录以查看站内通知</EmptyTitle>
                <EmptyDescription>
                  登录后即可实时接收讨论回复提醒、课程订单开通通知与售后服务进展。
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent className="flex-row justify-center gap-2">
                <Button asChild>
                  <Link href={loginHref}>前往登录</Link>
                </Button>
                <Button variant="secondary" asChild>
                  <Link href={registerHref}>注册新账号</Link>
                </Button>
              </EmptyContent>
            </Empty>
          </CardContent>
        </Card>
      </Layout>
    );
  }

  let initialNotifications: CommunityApi.Notification[] = [];
  let initialTotal = 0;
  let initialError: string | null = null;

  try {
    const options = await getServerRequestOptions();
    const resp = await communityListMyNotifications(
      {page: 1, pageSize: 50},
      options,
    );
    initialNotifications = resp.items ?? [];
    initialTotal = resp.total ?? initialNotifications.length;
  } catch (err: unknown) {
    initialError = getErrorMessage(err, '站内通知列表暂时无法加载');
  }

  return (
    <NotificationsView
      initialNotifications={initialNotifications}
      initialTotal={initialTotal}
      initialError={initialError}
    />
  );
}
