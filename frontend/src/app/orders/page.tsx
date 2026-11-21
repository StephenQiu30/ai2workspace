import type {Metadata} from 'next';
import Link from 'next/link';
import {
  AlertCircleIcon,
  ArrowRightIcon,
  LockIcon,
  ReceiptTextIcon,
} from 'lucide-react';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {getOrderStatusMeta} from '@/features/commerce/checkout';
import {getServerSession} from '@/lib/auth/session';
import {
  buildLoginHref,
  formatDateTime,
  formatPriceCny,
  getErrorMessage,
} from '@/lib/format';
import {getServerRequestOptions} from '@/lib/request/server';
import {commerceListOrders} from '@/services/generated/commerce/commerce';

export const metadata: Metadata = {
  title: '我的订单',
  description: '查看您在 ai2workspace 的课程购买订单、支付状态与退款售后进度。',
};

export const dynamic = 'force-dynamic';

export default async function OrdersPage() {
  const user = await getServerSession();

  if (!user) {
    return (
      <section className="mx-auto w-full max-w-4xl space-y-6">
        <header className="space-y-1.5">
          <h1 className="text-[28px] leading-[40px] font-medium text-foreground">
            我的订单
          </h1>
          <p className="text-sm leading-6 text-muted-foreground">
            查看您在 ai2workspace
            购买的所有课程订单、权益生效状态与退款售后进度。
          </p>
        </header>

        <Card>
          <CardContent className="py-12">
            <Empty>
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <LockIcon />
                </EmptyMedia>
                <EmptyTitle>请先登录查看订单</EmptyTitle>
                <EmptyDescription>
                  登录后即可查看历史课程购买记录、继续完成待支付订单或申请 7×24
                  小时退款售后。
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent className="flex-row justify-center">
                <Button asChild>
                  <Link href={buildLoginHref('/orders')}>前往登录</Link>
                </Button>
                <Button asChild variant="secondary">
                  <Link href="/courses">浏览课程目录</Link>
                </Button>
              </EmptyContent>
            </Empty>
          </CardContent>
        </Card>
      </section>
    );
  }

  let orders: CommerceApi.OrderResponse[] = [];
  let loadError: string | null = null;

  try {
    const options = await getServerRequestOptions();
    const resp = await commerceListOrders({page: 1, limit: 50}, options);
    orders = resp.items ?? [];
  } catch (err: unknown) {
    loadError = getErrorMessage(err, '加载订单列表失败，请稍后重试');
  }

  return (
    <section className="mx-auto w-full max-w-5xl space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div className="space-y-1.5">
          <h1 className="text-[28px] leading-[40px] font-medium text-foreground">
            我的订单
          </h1>
          <p className="text-sm leading-6 text-muted-foreground">
            查看您在 ai2workspace 购买的所有课程订单、支付核验与 7×24
            小时退款进度。
          </p>
        </div>
        <Button asChild variant="secondary" size="sm">
          <Link href="/courses">
            浏览更多课程
            <ArrowRightIcon />
          </Link>
        </Button>
      </div>

      {loadError ? (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>订单列表加载异常</AlertTitle>
          <AlertDescription>{loadError}</AlertDescription>
        </Alert>
      ) : null}

      {orders.length === 0 ? (
        <Card>
          <CardContent className="py-12">
            <Empty>
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <ReceiptTextIcon />
                </EmptyMedia>
                <EmptyTitle>暂无任何订单记录</EmptyTitle>
                <EmptyDescription>
                  您还没有创建过课程购买订单。前往课程目录挑选感兴趣的课程开始学习吧。
                </EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                <Button asChild size="sm">
                  <Link href="/courses">前往课程目录选购</Link>
                </Button>
              </EmptyContent>
            </Empty>
          </CardContent>
        </Card>
      ) : (
        <>
          {/* 桌面端表格 */}
          <div className="hidden md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>订单号</TableHead>
                  <TableHead>课程标题</TableHead>
                  <TableHead>金额</TableHead>
                  <TableHead>订单状态</TableHead>
                  <TableHead>创建时间</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {orders.map((order) => {
                  const statusMeta = getOrderStatusMeta(order.status);
                  const titleText =
                    order.courseTitle?.trim() || order.courseId || '未命名课程';
                  return (
                    <TableRow key={order.id}>
                      <TableCell className="font-mono text-xs text-muted-foreground">
                        {order.id}
                      </TableCell>
                      <TableCell>
                        <div className="space-y-0.5">
                          <div className="font-medium text-foreground">
                            {titleText}
                          </div>
                          {order.courseTitle && order.courseId ? (
                            <div className="font-mono text-[11px] text-muted-foreground">
                              {order.courseId}
                            </div>
                          ) : null}
                        </div>
                      </TableCell>
                      <TableCell className="font-medium text-foreground whitespace-nowrap">
                        {formatPriceCny(order.amountMinor, false)}
                      </TableCell>
                      <TableCell>
                        <Badge variant={statusMeta.variant}>
                          {statusMeta.label}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                        {formatDateTime(order.createdAt)}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button asChild variant="secondary" size="xs">
                          <Link href={`/orders/${order.id}`}>查看详情</Link>
                        </Button>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>

          {/* 移动端无边框卡片 */}
          <div className="grid gap-3 md:hidden">
            {orders.map((order) => {
              const statusMeta = getOrderStatusMeta(order.status);
              const titleText =
                order.courseTitle?.trim() || order.courseId || '未命名课程';
              return (
                <Card key={order.id} size="sm">
                  <CardHeader>
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0 space-y-0.5">
                        <CardTitle className="truncate">{titleText}</CardTitle>
                        <CardDescription className="truncate font-mono text-xs">
                          订单号：{order.id}
                        </CardDescription>
                      </div>
                      <Badge variant={statusMeta.variant}>
                        {statusMeta.label}
                      </Badge>
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-1.5 text-xs">
                    <div className="flex items-center justify-between">
                      <span className="text-muted-foreground">订单金额</span>
                      <span className="text-sm font-medium text-foreground">
                        {formatPriceCny(order.amountMinor, false)}
                      </span>
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-muted-foreground">创建时间</span>
                      <span className="text-muted-foreground">
                        {formatDateTime(order.createdAt)}
                      </span>
                    </div>
                  </CardContent>
                  <CardFooter className="justify-end">
                    <Button asChild variant="secondary" size="sm">
                      <Link href={`/orders/${order.id}`}>查看详情</Link>
                    </Button>
                  </CardFooter>
                </Card>
              );
            })}
          </div>
        </>
      )}
    </section>
  );
}
