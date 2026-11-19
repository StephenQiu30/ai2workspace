import Link from 'next/link';
import {commerceListOrders} from '@/services/generated/commerce/commerce';
import {formatPrice} from '@/features/commerce/checkout';
import {Card, CardContent, CardHeader} from '@/components/ui/card';
import {Button} from '@/components/ui/button';

export default async function OrdersPage() {
  let orders: CommerceApi.OrderResponse[] = [];
  try {
    const resp = await commerceListOrders({page: 1, limit: 50});
    orders = resp.items || [];
  } catch {
    orders = [];
  }

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">我的订单</h1>
          <p className="text-xs text-muted-foreground mt-1">
            查看您在 ai2workspace 购买的所有课程订单与退款进度
          </p>
        </div>
        <Link href="/courses">
          <Button variant="outline" size="sm">
            浏览更多课程
          </Button>
        </Link>
      </div>

      {orders.length === 0 ? (
        <Card className="text-center py-12">
          <CardContent className="space-y-3">
            <p className="text-sm text-muted-foreground">暂无任何订单记录</p>
            <Link href="/courses">
              <Button size="sm">前往课程广场选购</Button>
            </Link>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {orders.map((o) => (
            <Card key={o.id} className="hover:shadow-sm transition-shadow">
              <CardHeader className="py-3 px-4 flex flex-row items-center justify-between border-b">
                <div className="text-xs font-mono text-muted-foreground">
                  订单号: {o.id}
                </div>
                <div className="text-xs font-semibold">
                  {o.status === 'paid' ? (
                    <span className="text-green-600">已支付</span>
                  ) : o.status === 'refunded' ? (
                    <span className="text-zinc-500">已退款</span>
                  ) : (
                    <span className="text-amber-600">待付款</span>
                  )}
                </div>
              </CardHeader>
              <CardContent className="py-3 px-4 flex items-center justify-between">
                <div>
                  <div className="text-xs text-muted-foreground">
                    课程编号: {o.courseId}
                  </div>
                  <div className="text-base font-bold text-primary mt-1">
                    {formatPrice(o.amountMinor || 0, o.currency)}
                  </div>
                </div>
                <Link href={`/orders/${o.id}`}>
                  <Button variant="outline" size="sm">
                    查看详情与退款
                  </Button>
                </Link>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
