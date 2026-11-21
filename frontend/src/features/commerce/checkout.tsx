'use client';

import {useState} from 'react';
import {useRouter} from 'next/navigation';
import {AlertCircleIcon, CheckCircle2Icon, CreditCardIcon} from 'lucide-react';
import {toast} from 'sonner';
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
import {Spinner} from '@/components/ui/spinner';
import {formatPriceCny, getErrorMessage} from '@/lib/format';
import {
  commerceCreateOrder,
  commerceCreatePaymentAttempt,
  commerceGetPaymentStatus,
} from '@/services/generated/commerce/commerce';

export type StatusBadgeVariant = 'default' | 'secondary' | 'destructive';

export interface StatusMeta {
  label: string;
  variant: StatusBadgeVariant;
}

export interface RefundExceptionOption {
  value: string;
  label: string;
  description: string;
}

export const REFUND_EXCEPTION_OPTIONS: readonly RefundExceptionOption[] = [
  {
    value: 'quality_issue',
    label: '严重课程质量问题（quality_issue）',
    description: '课程核心章节或关键资源存在严重错误、缺失且未及时修复',
  },
  {
    value: 'delivery_unavailable',
    label: '课程交付不可用（delivery_unavailable）',
    description: '因平台或课程故障导致无法正常访问已购课程内容',
  },
  {
    value: 'duplicate_charge',
    label: '重复扣款或异常计费（duplicate_charge）',
    description: '同一课程发生重复支付或渠道异常多扣款项',
  },
  {
    value: 'legal_entitlement',
    label: '法定退费权益（legal_entitlement）',
    description: '依据适用法律法规享有的法定退款或争议处理情形',
  },
];

export function formatPrice(amountMinor: number, currency = 'CNY'): string {
  const yuan = (amountMinor / 100).toFixed(2);
  return currency === 'CNY' ? `¥${yuan}` : `${currency} ${yuan}`;
}

export function isRefundEligible(
  status: string,
  paidAt?: string | null,
  now = new Date(),
): boolean {
  if (status !== 'paid' || !paidAt) {
    return false;
  }
  const paidTime = new Date(paidAt).getTime();
  if (Number.isNaN(paidTime)) {
    return false;
  }
  const deadline = paidTime + 7 * 24 * 60 * 60 * 1000;
  return now.getTime() <= deadline;
}

export function getRefundWindowDeadline(paidAt?: string | null): Date | null {
  if (!paidAt) {
    return null;
  }
  const paidTime = new Date(paidAt).getTime();
  if (Number.isNaN(paidTime)) {
    return null;
  }
  return new Date(paidTime + 7 * 24 * 60 * 60 * 1000);
}

export function getOrderStatusMeta(status?: string): StatusMeta {
  switch (status) {
    case 'paid':
      return {label: '已支付', variant: 'default'};
    case 'pending_payment':
    case 'pending':
      return {label: '待支付', variant: 'secondary'};
    case 'cancelled':
      return {label: '已取消', variant: 'secondary'};
    case 'refunded':
      return {label: '已退款', variant: 'secondary'};
    default:
      return {label: status || '未知状态', variant: 'secondary'};
  }
}

export function getRefundApplicationStateMeta(state?: string): StatusMeta {
  switch (state) {
    case 'pending_review':
    case 'pending':
      return {label: '待审核', variant: 'default'};
    case 'approved':
      return {label: '已批准', variant: 'secondary'};
    case 'rejected':
      return {label: '已驳回', variant: 'destructive'};
    case 'cancelled':
      return {label: '已撤回', variant: 'secondary'};
    default:
      return {label: state || '未知审核状态', variant: 'secondary'};
  }
}

export function getRefundChannelStateMeta(state?: string): StatusMeta {
  switch (state) {
    case 'submitting':
      return {label: '渠道提交中', variant: 'secondary'};
    case 'success':
      return {label: '原路退款成功', variant: 'default'};
    case 'failed':
      return {label: '渠道退款失败', variant: 'destructive'};
    case 'unknown':
      return {label: '渠道状态待核实', variant: 'secondary'};
    default:
      return {label: state || '未发起渠道退款', variant: 'secondary'};
  }
}

export function getRefundExceptionLabel(exceptionType?: string): string {
  if (!exceptionType) {
    return '7×24 小时标准窗口内退款';
  }
  const found = REFUND_EXCEPTION_OPTIONS.find(
    (item) => item.value === exceptionType,
  );
  return found ? found.label : exceptionType;
}

export interface CheckoutProps {
  readonly courseId: string;
  readonly courseTitle: string;
  readonly amountMinor: number;
  readonly currency?: string;
}

export function Checkout({
  courseId,
  courseTitle,
  amountMinor,
  currency = 'CNY',
}: CheckoutProps) {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [checkingStatus, setCheckingStatus] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [createdOrder, setCreatedOrder] =
    useState<CommerceApi.OrderResponse | null>(null);
  const [paymentAttempt, setPaymentAttempt] =
    useState<CommerceApi.PaymentAttemptResponse | null>(null);

  async function handleCreateOrderAndInitiatePayment() {
    setLoading(true);
    setErrorMessage(null);

    try {
      const idempotencyKey = `idemp_${courseId}_${Date.now()}`;
      const order = await commerceCreateOrder({
        courseId,
        idempotencyKey,
      });
      setCreatedOrder(order);

      if (order.id) {
        const attempt = await commerceCreatePaymentAttempt(
          {id: order.id},
          {channel: 'alipay_cny'},
        );
        setPaymentAttempt(attempt);
      }
      toast.success('订单已创建并提交支付渠道受理');
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '创建订单或发起支付失败');
      if (msg.includes('already has active entitlement')) {
        setErrorMessage('您已经拥有该课程的有效学习权益，无需重复购买。');
      } else {
        setErrorMessage(msg);
      }
    } finally {
      setLoading(false);
    }
  }

  async function handleRefreshPaymentStatus() {
    if (!createdOrder?.id) {
      return;
    }
    setCheckingStatus(true);
    setErrorMessage(null);
    try {
      const statusResp = await commerceGetPaymentStatus({id: createdOrder.id});
      if (statusResp.orderStatus) {
        setCreatedOrder((prev) =>
          prev ? {...prev, status: statusResp.orderStatus} : prev,
        );
      }
      if (statusResp.latestAttempt) {
        setPaymentAttempt(statusResp.latestAttempt);
      }
      toast.success(statusResp.message || '已同步最新支付状态');
      if (statusResp.orderStatus === 'paid') {
        router.refresh();
      }
    } catch (err: unknown) {
      setErrorMessage(getErrorMessage(err, '刷新支付状态失败'));
    } finally {
      setCheckingStatus(false);
    }
  }

  const isPaid = createdOrder?.status === 'paid';
  const serverAmount = createdOrder?.amountMinor ?? amountMinor;
  const serverCurrency = createdOrder?.currency ?? currency;

  return (
    <Card className="mx-auto max-w-md">
      <CardHeader>
        <div className="flex items-start justify-between gap-2">
          <div className="space-y-1">
            <CardTitle>确认课程购买</CardTitle>
            <CardDescription>
              人民币单课结算 · 7×24 小时全额原路退款保障
            </CardDescription>
          </div>
          {createdOrder ? (
            <Badge variant={getOrderStatusMeta(createdOrder.status).variant}>
              {getOrderStatusMeta(createdOrder.status).label}
            </Badge>
          ) : null}
        </div>
      </CardHeader>
      <CardContent className="space-y-4 text-sm">
        <div className="flex items-center justify-between py-2">
          <span className="text-muted-foreground">所购课程</span>
          <span className="font-medium text-foreground">{courseTitle}</span>
        </div>

        <div className="flex items-center justify-between py-2">
          <span className="text-muted-foreground">服务端核定金额</span>
          <span className="text-lg font-medium text-foreground">
            {serverCurrency === 'CNY'
              ? formatPriceCny(serverAmount, false)
              : formatPrice(serverAmount, serverCurrency)}
          </span>
        </div>

        <div className="rounded-lg bg-secondary p-3 text-xs leading-relaxed text-muted-foreground">
          支付确认以服务端渠道签名为准，确认成功后立即激活课程学习权益。自课程实际可访问起
          7×24 小时内，可随时申请全额原路退款。
        </div>

        {errorMessage ? (
          <Alert variant="destructive">
            <AlertCircleIcon />
            <AlertTitle>操作提示</AlertTitle>
            <AlertDescription>{errorMessage}</AlertDescription>
          </Alert>
        ) : null}

        {createdOrder ? (
          <Alert role="status">
            <CheckCircle2Icon />
            <AlertTitle>
              {isPaid ? '支付已确认，课程权益已生效' : '支付单已提交渠道受理'}
            </AlertTitle>
            <AlertDescription className="space-y-2 text-xs">
              <div>
                订单号：
                <span className="font-mono text-foreground">
                  {createdOrder.id}
                </span>
              </div>
              {paymentAttempt ? (
                <div className="space-y-1">
                  <div>
                    渠道流水号：
                    <span className="font-mono text-foreground">
                      {paymentAttempt.channelOrderId || paymentAttempt.id}
                    </span>
                  </div>
                  {paymentAttempt.paymentAction ? (
                    <div className="break-all text-muted-foreground">
                      渠道受理说明：{paymentAttempt.paymentAction}
                    </div>
                  ) : null}
                </div>
              ) : null}
            </AlertDescription>
          </Alert>
        ) : null}
      </CardContent>
      <CardFooter className="flex flex-col gap-2">
        {!createdOrder ? (
          <Button
            type="button"
            className="w-full"
            onClick={() => void handleCreateOrderAndInitiatePayment()}
            disabled={loading}
          >
            {loading ? <Spinner /> : <CreditCardIcon />}
            {loading
              ? '正在创建订单并请求支付渠道…'
              : `确认下单并发起支付（${formatPriceCny(serverAmount, false)}）`}
          </Button>
        ) : (
          <div className="flex w-full flex-wrap items-center justify-end gap-2">
            {!isPaid ? (
              <Button
                type="button"
                variant="secondary"
                size="sm"
                disabled={checkingStatus}
                onClick={() => void handleRefreshPaymentStatus()}
              >
                {checkingStatus ? <Spinner /> : null}
                刷新支付状态
              </Button>
            ) : (
              <Button
                type="button"
                size="sm"
                onClick={() => router.push(`/courses/${courseId}`)}
              >
                进入课程学习
              </Button>
            )}
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => router.push(`/orders/${createdOrder.id}`)}
            >
              前往订单详情
            </Button>
          </div>
        )}
      </CardFooter>
    </Card>
  );
}
