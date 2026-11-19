'use client';

import React, {useState} from 'react';
import {useRouter} from 'next/navigation';
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
  commerceCreateOrder,
  commerceCreatePaymentAttempt,
  commerceHandlePaymentWebhook,
} from '@/services/generated/commerce/commerce';

export function formatPrice(amountMinor: number, currency = 'CNY'): string {
  const yuan = (amountMinor / 100).toFixed(2);
  return currency === 'CNY' ? `¥${yuan}` : `${currency} ${yuan}`;
}

export function isRefundEligible(
  status: string,
  paidAt?: string | null,
  now = new Date(),
): boolean {
  if (status !== 'paid' || !paidAt) return false;
  const paidTime = new Date(paidAt).getTime();
  const deadline = paidTime + 7 * 24 * 60 * 60 * 1000;
  return now.getTime() <= deadline;
}

export interface CheckoutProps {
  courseId: string;
  courseTitle: string;
  amountMinor: number;
  currency?: string;
}

export function Checkout({
  courseId,
  courseTitle,
  amountMinor,
  currency = 'CNY',
}: CheckoutProps) {
  const router = useRouter();
  const [loading, setLoading] = useState<boolean>(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [createdOrder, setCreatedOrder] =
    useState<CommerceApi.OrderResponse | null>(null);
  const [paymentSuccess, setPaymentSuccess] = useState<boolean>(false);

  const handleCreateOrderAndPay = async () => {
    setLoading(true);
    setErrorMessage(null);

    try {
      // 1. Create order with client idempotency key
      const idempotencyKey = `idemp_${courseId}_${Date.now()}`;
      const order = await commerceCreateOrder({
        courseId,
        idempotencyKey,
      });
      setCreatedOrder(order);

      // 2. Initiate payment attempt
      await commerceCreatePaymentAttempt(
        {id: order.id || ''},
        {channel: 'mock_sandbox'},
      );

      // 3. Complete payment in sandbox gateway
      await commerceHandlePaymentWebhook({
        orderId: order.id || '',
        channel: 'mock_sandbox',
        channelOrderId: `sandbox_tx_${Date.now()}`,
        amountMinor: order.amountMinor || amountMinor,
        currency: order.currency || currency,
        signature: 'sandbox_valid_sig',
      });

      setPaymentSuccess(true);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '下单支付失败';
      if (msg.includes('already has active entitlement')) {
        setErrorMessage('您已经拥有该课程的有效学习权益，无需重复购买。');
      } else {
        setErrorMessage(msg);
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card className="max-w-md mx-auto shadow-md">
      <CardHeader>
        <CardTitle className="text-lg font-bold">确认课程购买</CardTitle>
        <CardDescription>
          人民币单课结算 · 7×24 小时全额原路退款保障
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4 text-sm">
        <div className="flex justify-between items-center py-2 border-b">
          <span className="text-muted-foreground">所购课程</span>
          <span className="font-medium text-foreground">{courseTitle}</span>
        </div>

        <div className="flex justify-between items-center py-2 border-b">
          <span className="text-muted-foreground">应付金额</span>
          <span className="text-xl font-bold text-primary">
            {formatPrice(amountMinor, currency)}
          </span>
        </div>

        <div className="text-xs text-muted-foreground bg-muted p-3 rounded leading-relaxed">
          购买后立即激活课程正文与高清视频播放权益。自实际交付生效起 7×24
          小时内，未退款前可随时申请全额原路退款，不扣减已学内容费用。
        </div>

        {errorMessage && (
          <div className="p-3 text-xs bg-destructive/10 text-destructive rounded font-medium">
            {errorMessage}
          </div>
        )}

        {paymentSuccess && createdOrder && (
          <div className="p-4 bg-green-50 border border-green-200 text-green-800 rounded space-y-2 text-xs">
            <div className="font-semibold text-sm">
              ✓ 支付成功，权益已实时激活！
            </div>
            <div>
              订单号: <span className="font-mono">{createdOrder.id}</span>
            </div>
            <div className="flex gap-2 pt-2">
              <Button
                size="sm"
                onClick={() => router.push(`/courses/${courseId}`)}
              >
                立即开始学习
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={() => router.push(`/orders/${createdOrder.id}`)}
              >
                查看订单详情
              </Button>
            </div>
          </div>
        )}
      </CardContent>
      {!paymentSuccess && (
        <CardFooter className="flex flex-col gap-2">
          <Button
            className="w-full"
            onClick={handleCreateOrderAndPay}
            disabled={loading}
          >
            {loading
              ? '正在处理安全支付...'
              : `确认并支付 ${formatPrice(amountMinor, currency)}`}
          </Button>
        </CardFooter>
      )}
    </Card>
  );
}
