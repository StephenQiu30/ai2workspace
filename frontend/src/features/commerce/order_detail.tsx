'use client';

import {useState} from 'react';
import Link from 'next/link';
import {useRouter} from 'next/navigation';
import {
  AlertCircleIcon,
  ArrowLeftIcon,
  BookOpenIcon,
  CheckCircle2Icon,
  CreditCardIcon,
  RefreshCwIcon,
  RotateCcwIcon,
  ShieldCheckIcon,
  XCircleIcon,
} from 'lucide-react';
import {toast} from 'sonner';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {Spinner} from '@/components/ui/spinner';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {Textarea} from '@/components/ui/textarea';
import {formatDateTime, formatPriceCny, getErrorMessage} from '@/lib/format';
import {
  commerceApplyRefund,
  commerceCancelOrder,
  commerceCancelRefund,
  commerceCreatePaymentAttempt,
  commerceGetOrder,
  commerceGetPaymentStatus,
  commerceListOrderRefunds,
  commerceListPaymentAttempts,
} from '@/services/generated/commerce/commerce';
import {
  REFUND_EXCEPTION_OPTIONS,
  getOrderStatusMeta,
  getRefundApplicationStateMeta,
  getRefundChannelStateMeta,
  getRefundExceptionLabel,
  getRefundWindowDeadline,
  isRefundEligible,
} from './checkout';

export interface OrderDetailProps {
  readonly order: CommerceApi.OrderResponse;
  readonly initialRefund?: CommerceApi.RefundResponse | null;
  readonly initialRefunds?: CommerceApi.RefundResponse[];
  readonly initialAttempts?: CommerceApi.PaymentAttemptResponse[];
}

export function OrderDetail({
  order: initialOrder,
  initialRefund = null,
  initialRefunds = [],
  initialAttempts = [],
}: OrderDetailProps) {
  const router = useRouter();
  const [order, setOrder] = useState<CommerceApi.OrderResponse>(initialOrder);
  const [refunds, setRefunds] = useState<CommerceApi.RefundResponse[]>(() => {
    if (initialRefunds.length > 0) {
      return initialRefunds;
    }
    if (initialRefund) {
      return [initialRefund];
    }
    return [];
  });
  const [attempts, setAttempts] =
    useState<CommerceApi.PaymentAttemptResponse[]>(initialAttempts);

  // Payment & order actions
  const [selectedChannel, setSelectedChannel] = useState<string>('alipay_cny');
  const [initiatingPayment, setInitiatingPayment] = useState(false);
  const [checkingPayment, setCheckingPayment] = useState(false);
  const [paymentMessage, setPaymentMessage] = useState<string | null>(null);
  const [cancelOrderOpen, setCancelOrderOpen] = useState(false);
  const [cancellingOrder, setCancellingOrder] = useState(false);

  // Refund Dialog state
  const [refundDialogOpen, setRefundDialogOpen] = useState(false);
  const [refundMode, setRefundMode] = useState<'standard' | 'exception'>(
    'standard',
  );
  const [exceptionType, setExceptionType] = useState<string>(
    REFUND_EXCEPTION_OPTIONS[0].value,
  );
  const [refundReason, setRefundReason] = useState<string>('');
  const [refundFormError, setRefundFormError] = useState<string | null>(null);
  const [applyingRefund, setApplyingRefund] = useState(false);

  // Cancel Refund AlertDialog state
  const [cancelRefundTarget, setCancelRefundTarget] =
    useState<CommerceApi.RefundResponse | null>(null);
  const [cancellingRefund, setCancellingRefund] = useState(false);

  const [errorBanner, setErrorBanner] = useState<string | null>(null);

  const isPendingPayment =
    order.status === 'pending_payment' || order.status === 'pending';
  const isPaid = order.status === 'paid';
  const isRefunded = order.status === 'refunded';

  const withinStandardWindow = isRefundEligible(
    order.status ?? '',
    order.paidAt,
  );
  const windowDeadline = getRefundWindowDeadline(order.paidAt);

  // Latest active or historical refund
  const activeRefund = refunds.find(
    (r) =>
      r.applicationState === 'pending_review' ||
      r.applicationState === 'pending' ||
      r.applicationState === 'approved',
  );
  const latestAttempt = attempts[0] ?? null;

  async function reloadOrderData() {
    if (!order.id) {
      return;
    }
    try {
      const [nextOrder, nextRefunds, nextAttempts] = await Promise.all([
        commerceGetOrder({id: order.id}),
        commerceListOrderRefunds({id: order.id}).catch(() => []),
        commerceListPaymentAttempts({id: order.id}).catch(() => []),
      ]);
      setOrder(nextOrder);
      setRefunds(nextRefunds ?? []);
      setAttempts(nextAttempts ?? []);
    } catch {
      // Fallback to router refresh
    }
    router.refresh();
  }

  async function handleCreatePaymentAttempt() {
    if (!order.id) {
      return;
    }
    setInitiatingPayment(true);
    setErrorBanner(null);
    setPaymentMessage(null);
    try {
      const attempt = await commerceCreatePaymentAttempt(
        {id: order.id},
        {
          channel: selectedChannel,
          idempotencyKey: `attempt_${order.id}_${Date.now()}`,
        },
      );
      setAttempts((prev) => [
        attempt,
        ...prev.filter((item) => item.id !== attempt.id),
      ]);
      setPaymentMessage(
        attempt.paymentAction
          ? `支付渠道已受理：${attempt.paymentAction}`
          : `已创建渠道支付流水（${attempt.channelOrderId || attempt.id}），请在完成支付后点击“刷新支付状态”同步服务端确认结果。`,
      );
      toast.success('已发起支付请求');
    } catch (err: unknown) {
      setErrorBanner(getErrorMessage(err, '发起支付失败'));
    } finally {
      setInitiatingPayment(false);
    }
  }

  async function handleQueryPaymentStatus() {
    if (!order.id) {
      return;
    }
    setCheckingPayment(true);
    setErrorBanner(null);
    try {
      const statusResp = await commerceGetPaymentStatus({id: order.id});
      setPaymentMessage(
        statusResp.message ||
          (statusResp.orderStatus === 'paid'
            ? '服务端已确认支付成功，课程学习权益已生效。'
            : '当前订单仍处于待支付状态，若已完成支付请稍后再次刷新核验。'),
      );
      if (statusResp.orderStatus === 'paid') {
        toast.success('支付状态已确认为已支付');
      } else {
        toast.message('已同步最新支付状态');
      }
      await reloadOrderData();
    } catch (err: unknown) {
      setErrorBanner(getErrorMessage(err, '查询支付状态失败'));
    } finally {
      setCheckingPayment(false);
    }
  }

  async function handleConfirmCancelOrder(
    e: React.MouseEvent<HTMLButtonElement>,
  ) {
    e.preventDefault();
    if (!order.id) {
      return;
    }
    setCancellingOrder(true);
    setErrorBanner(null);
    try {
      const updated = await commerceCancelOrder({id: order.id});
      setOrder(updated);
      setCancelOrderOpen(false);
      toast.success('订单已取消');
      await reloadOrderData();
    } catch (err: unknown) {
      setErrorBanner(getErrorMessage(err, '取消订单失败'));
    } finally {
      setCancellingOrder(false);
    }
  }

  function openRefundDialog() {
    setRefundMode(withinStandardWindow ? 'standard' : 'exception');
    setExceptionType(REFUND_EXCEPTION_OPTIONS[0].value);
    setRefundReason('');
    setRefundFormError(null);
    setRefundDialogOpen(true);
  }

  async function handleApplyRefundSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!order.id) {
      return;
    }
    const trimmedReason = refundReason.trim();
    if (!trimmedReason) {
      setRefundFormError('请填写具体的退款原因说明');
      return;
    }

    const useException = !withinStandardWindow || refundMode === 'exception';
    if (useException && !exceptionType) {
      setRefundFormError('超出 7×24 小时标准窗口须选择适用的人工审核情形');
      return;
    }

    setApplyingRefund(true);
    setRefundFormError(null);
    try {
      const created = await commerceApplyRefund(
        {id: order.id},
        {
          reason: trimmedReason,
          exceptionType: useException ? exceptionType : undefined,
        },
      );
      setRefunds((prev) => [
        created,
        ...prev.filter((item) => item.id !== created.id),
      ]);
      setRefundDialogOpen(false);
      toast.success('退款申请已提交');
      await reloadOrderData();
    } catch (err: unknown) {
      setRefundFormError(getErrorMessage(err, '提交退款申请失败'));
    } finally {
      setApplyingRefund(false);
    }
  }

  async function handleConfirmCancelRefund(
    e: React.MouseEvent<HTMLButtonElement>,
  ) {
    e.preventDefault();
    if (!cancelRefundTarget?.id) {
      return;
    }
    setCancellingRefund(true);
    setErrorBanner(null);
    try {
      await commerceCancelRefund(
        {id: cancelRefundTarget.id},
        {expectedVersion: cancelRefundTarget.version ?? 1},
      );
      setCancelRefundTarget(null);
      toast.success('已撤回退款申请，课程学习权益继续保持有效');
      await reloadOrderData();
    } catch (err: unknown) {
      setErrorBanner(getErrorMessage(err, '撤回退款申请失败'));
      await reloadOrderData();
    } finally {
      setCancellingRefund(false);
    }
  }

  const statusMeta = getOrderStatusMeta(order.status);
  const courseDisplayName =
    order.courseTitle?.trim() || order.courseId || '未知课程';

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <Button asChild variant="ghost" size="xs">
              <Link href="/orders">
                <ArrowLeftIcon />
                返回我的订单
              </Link>
            </Button>
          </div>
          <h1 className="text-[28px] leading-[40px] font-medium text-foreground">
            订单详情与售后
          </h1>
          <p className="font-mono text-xs text-muted-foreground">
            订单编号：{order.id}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={statusMeta.variant}>{statusMeta.label}</Badge>
          {order.courseId ? (
            <Button
              asChild
              variant={isPaid ? 'default' : 'secondary'}
              size="sm"
            >
              <Link href={`/courses/${order.courseId}`}>
                <BookOpenIcon />
                {isPaid ? '进入课程学习' : '查看课程页面'}
              </Link>
            </Button>
          ) : null}
        </div>
      </div>

      {errorBanner ? (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>操作提示</AlertTitle>
          <AlertDescription>{errorBanner}</AlertDescription>
        </Alert>
      ) : null}

      {paymentMessage ? (
        <Alert role="status">
          <CheckCircle2Icon />
          <AlertTitle>支付渠道与核验反馈</AlertTitle>
          <AlertDescription>{paymentMessage}</AlertDescription>
        </Alert>
      ) : null}

      {/* 订单核心信息卡片 */}
      <Card>
        <CardHeader>
          <div className="flex items-start justify-between gap-2">
            <div className="space-y-1">
              <CardTitle>{courseDisplayName}</CardTitle>
              <CardDescription>
                课程标识：<span className="font-mono">{order.courseId}</span>
                {order.policyVersion
                  ? ` · 退款政策版本 ${order.policyVersion}`
                  : ''}
              </CardDescription>
            </div>
            <div className="text-right">
              <div className="text-xs text-muted-foreground">
                服务端核定金额
              </div>
              <div className="text-xl font-medium text-foreground">
                {formatPriceCny(order.amountMinor, false)}
              </div>
            </div>
          </div>
        </CardHeader>

        <CardContent className="space-y-4 text-sm">
          <div className="grid gap-4 rounded-lg bg-secondary p-4 sm:grid-cols-3">
            <div>
              <div className="text-xs text-muted-foreground">订单状态</div>
              <div className="mt-1 font-medium text-foreground">
                {statusMeta.label}
              </div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">创建时间</div>
              <div className="mt-1 text-foreground">
                {formatDateTime(order.createdAt)}
              </div>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">支付确认时间</div>
              <div className="mt-1 text-foreground">
                {order.paidAt ? formatDateTime(order.paidAt) : '尚未支付'}
              </div>
            </div>
          </div>

          <div className="rounded-lg bg-secondary p-4 text-xs leading-6 text-muted-foreground">
            <div className="font-medium text-foreground">
              7×24 小时全额原路退款政策说明
            </div>
            <p className="mt-1">
              订单金额由服务端统一核定。支付经渠道确认后立即授予课程学习权益；自课程实际可访问起
              7×24 小时内，可申请全额原路退款。超出 7×24
              小时窗口后，如存在严重课程质量问题、交付不可用、重复扣款或法定退费权益，仍可提交人工异常退款审核。在原路退款渠道确认成功（channelState=success）前，您的既有学习权益保持有效。
            </p>
          </div>
        </CardContent>
      </Card>

      {/* 待支付状态操作卡片 */}
      {isPendingPayment ? (
        <Card>
          <CardHeader>
            <CardTitle>待支付结算与状态核验</CardTitle>
            <CardDescription>
              请选择支付方式发起渠道支付，或点击“刷新支付状态”主动向服务端查单恢复。
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
              <div className="w-full sm:max-w-xs space-y-1.5">
                <span className="text-xs font-medium text-foreground">
                  支付渠道
                </span>
                <Select
                  value={selectedChannel}
                  onValueChange={setSelectedChannel}
                >
                  <SelectTrigger aria-label="选择支付渠道">
                    <SelectValue placeholder="选择支付渠道" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="alipay_cny">
                      支付宝人民币结算（alipay_cny）
                    </SelectItem>
                    <SelectItem value="wechat_cny">
                      微信支付人民币结算（wechat_cny）
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className="flex flex-wrap items-center gap-2">
                <Button
                  type="button"
                  disabled={initiatingPayment}
                  onClick={() => void handleCreatePaymentAttempt()}
                >
                  {initiatingPayment ? <Spinner /> : <CreditCardIcon />}
                  {initiatingPayment
                    ? '发起支付中…'
                    : `发起支付（${formatPriceCny(order.amountMinor, false)}）`}
                </Button>

                <Button
                  type="button"
                  variant="secondary"
                  disabled={checkingPayment}
                  onClick={() => void handleQueryPaymentStatus()}
                >
                  <RefreshCwIcon
                    className={checkingPayment ? 'animate-spin' : undefined}
                  />
                  刷新支付状态
                </Button>

                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => setCancelOrderOpen(true)}
                >
                  <XCircleIcon />
                  取消订单
                </Button>
              </div>
            </div>

            {latestAttempt ? (
              <div className="rounded-lg bg-secondary p-3.5 text-xs space-y-1.5">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="font-medium text-foreground">
                    最近一次支付流水：
                    {latestAttempt.channelOrderId || latestAttempt.id}
                  </span>
                  <Badge variant="secondary">
                    渠道：{latestAttempt.channel || '默认渠道'} · 状态：
                    {latestAttempt.status || 'initiated'}
                  </Badge>
                </div>
                {latestAttempt.paymentAction ? (
                  <div className="text-muted-foreground break-all">
                    渠道受理指引：{latestAttempt.paymentAction}
                  </div>
                ) : null}
              </div>
            ) : null}
          </CardContent>
        </Card>
      ) : null}

      {/* 已支付状态：权益生效与退款窗口卡片 */}
      {isPaid || isRefunded ? (
        <Card>
          <CardHeader>
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div className="space-y-1">
                <CardTitle>学习权益与退款售后</CardTitle>
                <CardDescription>
                  {isRefunded
                    ? '该订单已完成全额原路退款，对应课程学习权益已同步撤销。'
                    : '您的课程学习权益当前处于有效状态。'}
                </CardDescription>
              </div>
              <Badge variant={isRefunded ? 'secondary' : 'default'}>
                {isRefunded ? '权益已撤销（已退款）' : '学习权益生效中'}
              </Badge>
            </div>
          </CardHeader>
          <CardContent className="space-y-4 text-sm">
            <div className="grid gap-3 rounded-lg bg-secondary p-4 sm:grid-cols-2">
              <div>
                <div className="text-xs text-muted-foreground">
                  7×24 小时标准退款窗口
                </div>
                <div className="mt-1 font-medium text-foreground">
                  {withinStandardWindow
                    ? '当前处于 7×24 小时标准全额退款窗口内'
                    : '已超出 7×24 小时标准窗口（支持异常情形人工申请）'}
                </div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground">
                  标准退款窗口截止时间
                </div>
                <div className="mt-1 text-foreground">
                  {windowDeadline
                    ? formatDateTime(windowDeadline.toISOString())
                    : '以课程首次可访问起 7×24 小时计算'}
                </div>
              </div>
            </div>
          </CardContent>
          {isPaid ? (
            <CardFooter className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <ShieldCheckIcon className="size-4" />
                <span>
                  {activeRefund
                    ? '当前存在处理中的退款申请，原路到账前学习权益保持有效'
                    : withinStandardWindow
                      ? '可随时发起窗口内标准退款或异常退款申请'
                      : '可选择异常原因提交窗口外人工退款审核'}
                </span>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                {order.courseId ? (
                  <Button asChild size="sm">
                    <Link href={`/courses/${order.courseId}`}>
                      <BookOpenIcon />
                      进入课程学习
                    </Link>
                  </Button>
                ) : null}
                {!activeRefund ? (
                  <Button
                    type="button"
                    variant="secondary"
                    size="sm"
                    onClick={openRefundDialog}
                  >
                    <RotateCcwIcon />
                    申请退款
                  </Button>
                ) : null}
              </div>
            </CardFooter>
          ) : null}
        </Card>
      ) : null}

      {/* 退款申请记录列表 */}
      {refunds.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>退款申请记录</CardTitle>
            <CardDescription>
              展示该笔订单的退款审核状态（applicationState）与支付渠道执行状态（channelState）。
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {refunds.map((item) => {
              const appMeta = getRefundApplicationStateMeta(
                item.applicationState,
              );
              const channelMeta = getRefundChannelStateMeta(item.channelState);
              const isPendingReview =
                item.applicationState === 'pending_review' ||
                item.applicationState === 'pending';

              return (
                <div
                  key={item.id}
                  className="rounded-lg bg-secondary p-4 space-y-3 text-xs"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="space-y-0.5">
                      <div className="font-medium text-sm text-foreground">
                        退款单号：<span className="font-mono">{item.id}</span>
                      </div>
                      <div className="text-muted-foreground">
                        申请时间：{formatDateTime(item.createdAt)} · 申请金额：
                        {formatPriceCny(
                          item.amountMinor ?? order.amountMinor,
                          false,
                        )}
                      </div>
                    </div>

                    <div className="flex flex-wrap items-center gap-1.5">
                      <Badge variant="secondary">
                        {item.isOutOfWindow ? '窗口外人工申请' : '7×24h 窗口内'}
                      </Badge>
                      <Badge variant={appMeta.variant}>
                        审核：{appMeta.label}
                      </Badge>
                      <Badge variant={channelMeta.variant}>
                        渠道：{channelMeta.label}
                      </Badge>
                    </div>
                  </div>

                  <div className="grid gap-2 sm:grid-cols-2">
                    <div>
                      <span className="text-muted-foreground">申请类型：</span>
                      <span className="text-foreground">
                        {getRefundExceptionLabel(item.exceptionType)}
                      </span>
                    </div>
                    <div>
                      <span className="text-muted-foreground">申请原因：</span>
                      <span className="text-foreground">
                        {item.reason || '未填写'}
                      </span>
                    </div>
                  </div>

                  {item.rejectionReason ? (
                    <div className="rounded-md bg-destructive-surface p-2.5 text-foreground">
                      审核驳回原因：{item.rejectionReason}
                    </div>
                  ) : null}

                  {item.channelFailureReason ? (
                    <div className="rounded-md bg-destructive-surface p-2.5 text-foreground">
                      渠道退款异常说明：{item.channelFailureReason}
                    </div>
                  ) : null}

                  {isPendingReview ? (
                    <div className="flex justify-end pt-1">
                      <Button
                        type="button"
                        variant="secondary"
                        size="xs"
                        onClick={() => setCancelRefundTarget(item)}
                      >
                        撤回退款申请
                      </Button>
                    </div>
                  ) : null}
                </div>
              );
            })}
          </CardContent>
        </Card>
      ) : null}

      {/* 支付尝试流水列表 */}
      {attempts.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>支付渠道流水记录</CardTitle>
            <CardDescription>
              每次发起支付都会生成独立的渠道受理流水，供对账与主动查单核验。
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>流水号</TableHead>
                  <TableHead>渠道单号</TableHead>
                  <TableHead>支付渠道</TableHead>
                  <TableHead>金额</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>创建时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {attempts.map((att) => (
                  <TableRow key={att.id}>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {att.id}
                    </TableCell>
                    <TableCell className="font-mono text-xs text-foreground">
                      {att.channelOrderId || '—'}
                    </TableCell>
                    <TableCell>{att.channel || '—'}</TableCell>
                    <TableCell>
                      {formatPriceCny(
                        att.amountMinor ?? order.amountMinor,
                        false,
                      )}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          att.status === 'succeeded' || att.status === 'paid'
                            ? 'default'
                            : 'secondary'
                        }
                      >
                        {att.status || 'initiated'}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                      {formatDateTime(att.createdAt)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ) : null}

      {/* 取消订单 AlertDialog */}
      <AlertDialog
        open={cancelOrderOpen}
        onOpenChange={(open) => {
          if (!open && !cancellingOrder) {
            setCancelOrderOpen(false);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认取消该待支付订单</AlertDialogTitle>
            <AlertDialogDescription>
              取消订单（{order.id}
              ）后，该订单将转为“已取消”终态且不再接受支付回调。若您后续仍需购买该课程，可重新在课程详情页下单。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={cancellingOrder}>
              保留订单
            </AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              disabled={cancellingOrder}
              onClick={(e) => void handleConfirmCancelOrder(e)}
            >
              {cancellingOrder ? <Spinner /> : null}
              {cancellingOrder ? '取消中…' : '确认取消订单'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 申请退款 Dialog */}
      <Dialog
        open={refundDialogOpen}
        onOpenChange={(open) => {
          if (!open && !applyingRefund) {
            setRefundDialogOpen(false);
            setRefundFormError(null);
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>申请订单退款</DialogTitle>
            <DialogDescription>
              {withinStandardWindow
                ? '当前订单处于课程首次可访问起 7×24 小时退款保障期内，支持标准全额原路退款。'
                : '当前订单已超出 7×24 小时标准退款窗口，请选择适用的人工审核情形并填写说明。'}
            </DialogDescription>
          </DialogHeader>

          <form
            onSubmit={(e) => void handleApplyRefundSubmit(e)}
            noValidate
            className="space-y-4"
          >
            <FieldGroup>
              {withinStandardWindow ? (
                <Field>
                  <FieldLabel htmlFor="refund-mode-select">申请类型</FieldLabel>
                  <Select
                    value={refundMode}
                    onValueChange={(v) =>
                      setRefundMode(v as 'standard' | 'exception')
                    }
                  >
                    <SelectTrigger id="refund-mode-select">
                      <SelectValue placeholder="选择退款申请类型" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="standard">
                        7×24 小时窗口内标准全额退款
                      </SelectItem>
                      <SelectItem value="exception">
                        专项异常情形退款（质量问题 / 重复扣款等）
                      </SelectItem>
                    </SelectContent>
                  </Select>
                  <FieldDescription>
                    标准窗口内默认无需额外证明即可申请全额原路退回。
                  </FieldDescription>
                </Field>
              ) : null}

              {!withinStandardWindow || refundMode === 'exception' ? (
                <Field>
                  <FieldLabel htmlFor="refund-exception-select">
                    窗口外 / 异常退款情形（exceptionType）
                  </FieldLabel>
                  <Select
                    value={exceptionType}
                    onValueChange={setExceptionType}
                  >
                    <SelectTrigger id="refund-exception-select">
                      <SelectValue placeholder="请选择适用情形" />
                    </SelectTrigger>
                    <SelectContent>
                      {REFUND_EXCEPTION_OPTIONS.map((opt) => (
                        <SelectItem key={opt.value} value={opt.value}>
                          {opt.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FieldDescription>
                    {
                      REFUND_EXCEPTION_OPTIONS.find(
                        (item) => item.value === exceptionType,
                      )?.description
                    }
                  </FieldDescription>
                </Field>
              ) : null}

              <Field data-invalid={Boolean(refundFormError)}>
                <FieldLabel htmlFor="refund-reason-textarea">
                  退款原因说明
                </FieldLabel>
                <Textarea
                  id="refund-reason-textarea"
                  value={refundReason}
                  onChange={(e) => setRefundReason(e.target.value)}
                  placeholder="请详细说明您申请退款的具体原因（如误购、课程章节问题或渠道异常等）"
                  aria-invalid={Boolean(refundFormError)}
                />
                <FieldDescription>
                  提交后将进入退款审核队列；在渠道原路退款确认成功前，课程学习权益继续保留。
                </FieldDescription>
                <FieldError>{refundFormError}</FieldError>
              </Field>
            </FieldGroup>

            <DialogFooter>
              <Button
                type="button"
                variant="secondary"
                disabled={applyingRefund}
                onClick={() => setRefundDialogOpen(false)}
              >
                取消
              </Button>
              <Button type="submit" disabled={applyingRefund}>
                {applyingRefund ? <Spinner /> : null}
                {applyingRefund ? '提交中…' : '确认提交退款申请'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* 撤回退款申请 AlertDialog */}
      <AlertDialog
        open={Boolean(cancelRefundTarget)}
        onOpenChange={(open) => {
          if (!open && !cancellingRefund) {
            setCancelRefundTarget(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认撤回退款申请</AlertDialogTitle>
            <AlertDialogDescription>
              撤回退款申请（单号 {cancelRefundTarget?.id}，版本 v
              {cancelRefundTarget?.version ?? 1}
              ）后，该退款单将变更为“已撤回”，您的课程学习权益继续保持有效。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={cancellingRefund}>
              暂不撤回
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={cancellingRefund}
              onClick={(e) => void handleConfirmCancelRefund(e)}
            >
              {cancellingRefund ? <Spinner /> : null}
              {cancellingRefund ? '撤回中…' : '确认撤回申请'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
