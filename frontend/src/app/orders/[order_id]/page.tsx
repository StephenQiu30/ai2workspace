import {Layout} from '@/components/layout/layout';
import type {Metadata} from 'next';
import Link from 'next/link';
import {notFound} from 'next/navigation';
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
import {OrderDetail} from '@/features/commerce/order_detail';
import {getServerSession} from '@/lib/auth/session';
import {buildLoginHref} from '@/lib/format';
import {getServerRequestOptions} from '@/lib/request/server';
import {
  commerceGetOrder,
  commerceListOrderRefunds,
  commerceListPaymentAttempts,
} from '@/services/generated/api/commerce';

export const metadata: Metadata = {
  title: '订单详情与售后',
  description:
    '查看订单服务端核定金额、支付状态核验、权益生效与 7×24 小时退款申请。',
};

export const dynamic = 'force-dynamic';

interface OrderDetailPageProps {
  params: Promise<{
    order_id: string;
  }>;
}

export default async function OrderDetailPage({params}: OrderDetailPageProps) {
  const [{order_id: orderId}, user] = await Promise.all([
    params,
    getServerSession(),
  ]);

  if (!user) {
    return (
      <Layout as="section" className="w-full space-y-6">
        <Card>
          <CardContent className="py-12">
            <Empty>
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <LockIcon />
                </EmptyMedia>
                <EmptyTitle>请先登录查看订单详情</EmptyTitle>
                <EmptyDescription>
                  订单详情、支付核验与退款售后仅对订单所属账号开放。
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent className="flex-row justify-center">
                <Button asChild>
                  <Link href={buildLoginHref(`/orders/${orderId}`)}>
                    前往登录
                  </Link>
                </Button>
                <Button asChild variant="secondary">
                  <Link href="/courses">返回课程目录</Link>
                </Button>
              </EmptyContent>
            </Empty>
          </CardContent>
        </Card>
      </Layout>
    );
  }

  let order: Api.OrderResponse | null = null;
  let initialRefunds: Api.RefundResponse[] = [];
  let initialAttempts: Api.PaymentAttemptResponse[] = [];

  try {
    const options = await getServerRequestOptions();
    const [orderResp, refundsResp, attemptsResp] = await Promise.all([
      commerceGetOrder({id: orderId}, options),
      commerceListOrderRefunds({id: orderId}, options).catch(() => []),
      commerceListPaymentAttempts({id: orderId}, options).catch(() => []),
    ]);
    order = orderResp;
    initialRefunds = refundsResp ?? [];
    initialAttempts = attemptsResp ?? [];
  } catch {
    notFound();
  }

  if (!order) {
    notFound();
  }

  return (
    <Layout as="section" className="w-full space-y-6">
      <OrderDetail
        order={order}
        initialRefunds={initialRefunds}
        initialAttempts={initialAttempts}
      />
    </Layout>
  );
}
