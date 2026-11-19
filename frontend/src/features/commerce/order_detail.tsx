'use client';

import React, {useState} from 'react';
import {useRouter} from 'next/navigation';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Label} from '@/components/ui/label';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {formatPrice, isRefundEligible} from './checkout';
import {commerceApplyRefund} from '@/services/generated/commerce/commerce';

export interface OrderDetailProps {
  order: CommerceApi.OrderResponse;
  initialRefund?: CommerceApi.RefundResponse | null;
}

export function OrderDetail({order, initialRefund}: OrderDetailProps) {
  const router = useRouter();
  const [refund, setRefund] = useState<CommerceApi.RefundResponse | null>(
    initialRefund || null,
  );
  const [showRefundForm, setShowRefundForm] = useState<boolean>(false);
  const [reason, setReason] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const eligible = isRefundEligible(order.status || '', order.paidAt);

  const handleApplyRefund = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!reason.trim()) {
      setErrorMessage('请填写退款申请原因');
      return;
    }

    setLoading(true);
    setErrorMessage(null);
    try {
      const resp = await commerceApplyRefund(
        {id: order.id || ''},
        {reason: reason.trim()},
      );
      setRefund(resp);
      setShowRefundForm(false);
      router.refresh();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '提交退款申请失败';
      setErrorMessage(msg);
    } finally {
      setLoading(false);
    }
  };

  const getStatusBadge = (status?: string) => {
    switch (status) {
      case 'paid':
        return (
          <span className="bg-green-100 text-green-800 px-2.5 py-0.5 rounded text-xs font-semibold">
            已支付
          </span>
        );
      case 'refunded':
        return (
          <span className="bg-zinc-100 text-zinc-800 px-2.5 py-0.5 rounded text-xs font-semibold">
            已全额退款
          </span>
        );
      case 'pending':
        return (
          <span className="bg-amber-100 text-amber-800 px-2.5 py-0.5 rounded text-xs font-semibold">
            待支付
          </span>
        );
      default:
        return (
          <span className="bg-muted text-muted-foreground px-2.5 py-0.5 rounded text-xs font-semibold">
            {status}
          </span>
        );
    }
  };

  return (
    <Card className="max-w-xl mx-auto shadow-sm">
      <CardHeader>
        <div className="flex justify-between items-start">
          <div>
            <CardTitle className="text-base font-semibold">订单详情</CardTitle>
            <CardDescription className="text-xs font-mono mt-1">
              {order.id}
            </CardDescription>
          </div>
          <div>{getStatusBadge(order.status)}</div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4 text-xs">
        <div className="grid grid-cols-2 gap-4 py-2 border-b">
          <div>
            <div className="text-muted-foreground">购买课程编号</div>
            <div className="font-mono mt-0.5 font-medium">{order.courseId}</div>
          </div>
          <div>
            <div className="text-muted-foreground">支付金额</div>
            <div className="font-semibold text-sm text-primary mt-0.5">
              {formatPrice(order.amountMinor || 0, order.currency)}
            </div>
          </div>
          <div>
            <div className="text-muted-foreground">下单时间</div>
            <div className="mt-0.5">
              {order.createdAt
                ? new Date(order.createdAt).toLocaleString()
                : '-'}
            </div>
          </div>
          <div>
            <div className="text-muted-foreground">支付完成时间</div>
            <div className="mt-0.5">
              {order.paidAt
                ? new Date(order.paidAt).toLocaleString()
                : '未支付'}
            </div>
          </div>
        </div>

        {/* Refund Status or Action */}
        {refund ? (
          <div className="bg-amber-50 border border-amber-200 p-3 rounded space-y-2">
            <div className="font-semibold text-amber-900 flex justify-between">
              <span>退款申请状态</span>
              <span className="font-mono">
                {refund.applicationState === 'pending'
                  ? '运营人工审核中'
                  : refund.applicationState === 'approved'
                    ? '已批准并原路退回'
                    : '已驳回'}
              </span>
            </div>
            <div className="text-amber-800">申请原因: {refund.reason}</div>
            {refund.rejectionReason && (
              <div className="text-destructive font-medium">
                驳回意见: {refund.rejectionReason}
              </div>
            )}
          </div>
        ) : eligible ? (
          <div className="pt-2">
            {!showRefundForm ? (
              <div className="flex justify-between items-center bg-muted/40 p-3 rounded">
                <span className="text-muted-foreground">
                  本订单处于 7×24 小时退款保障期内
                </span>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setShowRefundForm(true)}
                >
                  申请退款
                </Button>
              </div>
            ) : (
              <form
                onSubmit={handleApplyRefund}
                className="space-y-3 bg-muted/20 p-3 rounded border"
              >
                <div className="font-medium">
                  提交 7×24 小时全额原路退款申请
                </div>
                <div className="space-y-1">
                  <Label htmlFor="refund-reason">退款原因说明</Label>
                  <Input
                    id="refund-reason"
                    placeholder="请输入退款原因（如内容不符、误购等）"
                    value={reason}
                    onChange={(e) => setReason(e.target.value)}
                    disabled={loading}
                    className="h-8 text-xs"
                  />
                </div>
                {errorMessage && (
                  <div className="text-destructive text-[11px]">
                    {errorMessage}
                  </div>
                )}
                <div className="flex gap-2 justify-end">
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    onClick={() => setShowRefundForm(false)}
                    disabled={loading}
                  >
                    取消
                  </Button>
                  <Button
                    type="submit"
                    size="sm"
                    variant="destructive"
                    disabled={loading}
                  >
                    {loading ? '提交中...' : '确认申请退款'}
                  </Button>
                </div>
              </form>
            )}
          </div>
        ) : order.status === 'paid' ? (
          <div className="text-muted-foreground bg-muted p-2.5 rounded text-[11px]">
            已超出 7×24
            小时无理由退款窗口。如遇严重课程质量问题，请联系人工客服。
          </div>
        ) : null}
      </CardContent>
      <CardFooter className="flex justify-between border-t py-3">
        <Button
          size="sm"
          variant="ghost"
          onClick={() => router.push('/orders')}
        >
          返回我的订单列表
        </Button>
        <Button
          size="sm"
          onClick={() => router.push(`/courses/${order.courseId}`)}
        >
          查看课程页面
        </Button>
      </CardFooter>
    </Card>
  );
}
