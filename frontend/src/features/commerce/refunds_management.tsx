'use client';

import {useMemo, useState} from 'react';
import {
  AlertCircleIcon,
  CheckCircle2Icon,
  FileSearchIcon,
  RefreshCwIcon,
  RotateCcwIcon,
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
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import {Input} from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import {Skeleton} from '@/components/ui/skeleton';
import {Spinner} from '@/components/ui/spinner';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {Tabs, TabsContent, TabsList, TabsTrigger} from '@/components/ui/tabs';
import {Textarea} from '@/components/ui/textarea';
import {formatDateTime, formatPriceCny, getErrorMessage} from '@/lib/format';
import {
  commerceConfirmRefundChannel,
  commerceGetRefund,
  commerceListRefunds,
  commerceReviewRefund,
} from '@/services/generated/commerce/commerce';
import {
  getRefundApplicationStateMeta,
  getRefundChannelStateMeta,
  getRefundExceptionLabel,
} from './checkout';

export interface RefundsManagementProps {
  readonly initialRefunds: CommerceApi.RefundListResponse;
  readonly initialError?: string | null;
}

type RefundTabKey =
  'all' | 'pending_review' | 'channel_processing' | 'completed';

interface ReviewTarget {
  refund: CommerceApi.RefundResponse;
  decision: 'approved' | 'rejected';
}

function isPendingReviewRefund(item: CommerceApi.RefundResponse): boolean {
  return (
    item.applicationState === 'pending_review' ||
    item.applicationState === 'pending'
  );
}

function isChannelProcessingRefund(item: CommerceApi.RefundResponse): boolean {
  return (
    item.applicationState === 'approved' &&
    (item.channelState === 'submitting' ||
      item.channelState === 'unknown' ||
      item.channelState === 'failed')
  );
}

function isCompletedRefund(item: CommerceApi.RefundResponse): boolean {
  return (
    item.applicationState === 'rejected' ||
    item.applicationState === 'cancelled' ||
    (item.applicationState === 'approved' && item.channelState === 'success')
  );
}

export function RefundsManagement({
  initialRefunds,
  initialError = null,
}: RefundsManagementProps) {
  const [activeTab, setActiveTab] = useState<RefundTabKey>('all');
  const [refunds, setRefunds] = useState<CommerceApi.RefundResponse[]>(
    initialRefunds.items ?? [],
  );
  const [loading, setLoading] = useState(false);
  const [errorBanner, setErrorBanner] = useState<string | null>(initialError);

  // Sheet inspection target
  const [selectedRefund, setSelectedRefund] =
    useState<CommerceApi.RefundResponse | null>(null);
  const [sheetOpen, setSheetOpen] = useState(false);

  // Review AlertDialog state (approve / reject)
  const [reviewTarget, setReviewTarget] = useState<ReviewTarget | null>(null);
  const [reviewReason, setReviewReason] = useState('');
  const [reviewReasonError, setReviewReasonError] = useState<string | null>(
    null,
  );
  const [submittingReview, setSubmittingReview] = useState(false);

  // Channel Confirm Dialog state
  const [channelDialogOpen, setChannelDialogOpen] = useState(false);
  const [channelTarget, setChannelTarget] =
    useState<CommerceApi.RefundResponse | null>(null);
  const [channelStatus, setChannelStatus] = useState<
    'success' | 'failed' | 'unknown'
  >('success');
  const [channelRefundId, setChannelRefundId] = useState('');
  const [channelFailureReason, setChannelFailureReason] = useState('');
  const [channelFormError, setChannelFormError] = useState<string | null>(null);
  const [submittingChannel, setSubmittingChannel] = useState(false);

  const counts = useMemo(() => {
    const pending = refunds.filter(isPendingReviewRefund).length;
    const processing = refunds.filter(isChannelProcessingRefund).length;
    const completed = refunds.filter(isCompletedRefund).length;
    return {
      all: refunds.length,
      pending_review: pending,
      channel_processing: processing,
      completed,
    };
  }, [refunds]);

  const filteredRefunds = useMemo(() => {
    switch (activeTab) {
      case 'pending_review':
        return refunds.filter(isPendingReviewRefund);
      case 'channel_processing':
        return refunds.filter(isChannelProcessingRefund);
      case 'completed':
        return refunds.filter(isCompletedRefund);
      default:
        return refunds;
    }
  }, [activeTab, refunds]);

  async function refreshRefunds(keepSelectedId?: string) {
    setLoading(true);
    setErrorBanner(null);
    try {
      const resp = await commerceListRefunds({page: 1, limit: 50});
      const nextItems = resp.items ?? [];
      setRefunds(nextItems);

      const targetId = keepSelectedId ?? selectedRefund?.id;
      if (targetId) {
        const updated =
          nextItems.find((item) => item.id === targetId) ??
          (await commerceGetRefund({id: targetId}).catch(() => null));
        if (updated) {
          setSelectedRefund(updated);
        }
      }
    } catch (err: unknown) {
      setErrorBanner(getErrorMessage(err, '刷新退款申请列表失败'));
    } finally {
      setLoading(false);
    }
  }

  function handleOpenSheet(item: CommerceApi.RefundResponse) {
    setSelectedRefund(item);
    setSheetOpen(true);
  }

  function openReviewDialog(
    refund: CommerceApi.RefundResponse,
    decision: 'approved' | 'rejected',
  ) {
    setReviewTarget({refund, decision});
    setReviewReason('');
    setReviewReasonError(null);
  }

  async function handleConfirmReview(e: React.MouseEvent<HTMLButtonElement>) {
    e.preventDefault();
    if (!reviewTarget?.refund.id) {
      return;
    }
    const trimmedReason = reviewReason.trim();
    if (reviewTarget.decision === 'rejected' && !trimmedReason) {
      setReviewReasonError('拒绝退款申请须填写明确的驳回理由，供申请人核查');
      return;
    }

    setSubmittingReview(true);
    setReviewReasonError(null);
    try {
      const updated = await commerceReviewRefund(
        {id: reviewTarget.refund.id},
        {
          decision: reviewTarget.decision,
          expectedVersion: reviewTarget.refund.version ?? 1,
          reason: trimmedReason || undefined,
        },
      );
      setSelectedRefund(updated);
      setReviewTarget(null);
      setReviewReason('');
      toast.success(
        reviewTarget.decision === 'approved'
          ? '已批准退款申请并转入渠道原路退款处理（学习权益在渠道确认成功前保持有效）'
          : '已驳回该笔退款申请',
      );
      await refreshRefunds(updated.id);
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '提交退款审核决定失败');
      setReviewReasonError(msg);
      if (msg.toLowerCase().includes('version') || msg.includes('冲突')) {
        await refreshRefunds(reviewTarget.refund.id);
      }
    } finally {
      setSubmittingReview(false);
    }
  }

  function openChannelConfirmDialog(refund: CommerceApi.RefundResponse) {
    setChannelTarget(refund);
    setChannelStatus('success');
    setChannelRefundId(
      refund.channelRefundId || `ch_ref_${refund.id || Date.now()}`,
    );
    setChannelFailureReason(refund.channelFailureReason || '');
    setChannelFormError(null);
    setChannelDialogOpen(true);
  }

  async function handleConfirmChannelSubmit(
    e: React.FormEvent<HTMLFormElement>,
  ) {
    e.preventDefault();
    if (!channelTarget?.id) {
      return;
    }
    if (
      (channelStatus === 'failed' || channelStatus === 'unknown') &&
      !channelFailureReason.trim()
    ) {
      setChannelFormError('当渠道结果为失败或未知时，请填写渠道反馈原因说明');
      return;
    }

    setSubmittingChannel(true);
    setChannelFormError(null);
    try {
      const updated = await commerceConfirmRefundChannel(
        {id: channelTarget.id},
        {
          status: channelStatus,
          channelRefundId: channelRefundId.trim() || undefined,
          failureReason: channelFailureReason.trim() || undefined,
          expectedVersion: channelTarget.version ?? 1,
        },
      );
      setSelectedRefund(updated);
      setChannelDialogOpen(false);
      setChannelTarget(null);
      toast.success(
        channelStatus === 'success'
          ? '已确认渠道原路退款成功，订单已标记为已退款并同步撤销学习权益'
          : '已登记渠道退款处理状态，学员学习权益继续保持有效',
      );
      await refreshRefunds(updated.id);
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '登记渠道退款结果失败');
      setChannelFormError(msg);
      if (msg.toLowerCase().includes('version') || msg.includes('冲突')) {
        await refreshRefunds(channelTarget.id);
      }
    } finally {
      setSubmittingChannel(false);
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div className="space-y-1.5">
          <h1 className="text-[28px] leading-[40px] font-medium text-foreground">
            退款处理
          </h1>
          <p className="text-sm leading-6 text-muted-foreground">
            审核 7×24
            小时标准退款与窗口外人工申请，跟踪并登记原路退款渠道执行结果；仅在渠道确认成功（success）时撤销课程学习权益。
          </p>
        </div>

        <Button
          type="button"
          variant="secondary"
          size="sm"
          disabled={loading}
          onClick={() => void refreshRefunds()}
        >
          <RefreshCwIcon className={loading ? 'animate-spin' : undefined} />
          刷新列表
        </Button>
      </div>

      {errorBanner ? (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>加载或操作异常</AlertTitle>
          <AlertDescription className="flex flex-wrap items-center justify-between gap-2">
            <span>{errorBanner}</span>
            <Button
              type="button"
              variant="secondary"
              size="xs"
              onClick={() => void refreshRefunds()}
            >
              重试
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      <Tabs
        value={activeTab}
        onValueChange={(v) => setActiveTab(v as RefundTabKey)}
      >
        <TabsList>
          <TabsTrigger value="all">
            全部
            <Badge variant="secondary" className="ml-1 h-5 px-1.5 text-[11px]">
              {counts.all}
            </Badge>
          </TabsTrigger>
          <TabsTrigger value="pending_review">
            待审核
            <Badge variant="secondary" className="ml-1 h-5 px-1.5 text-[11px]">
              {counts.pending_review}
            </Badge>
          </TabsTrigger>
          <TabsTrigger value="channel_processing">
            渠道处理中
            <Badge variant="secondary" className="ml-1 h-5 px-1.5 text-[11px]">
              {counts.channel_processing}
            </Badge>
          </TabsTrigger>
          <TabsTrigger value="completed">
            已完成
            <Badge variant="secondary" className="ml-1 h-5 px-1.5 text-[11px]">
              {counts.completed}
            </Badge>
          </TabsTrigger>
        </TabsList>

        <TabsContent value={activeTab} className="space-y-4 pt-4">
          {loading ? (
            <div className="space-y-2.5">
              <Skeleton className="h-12 w-full rounded-lg" />
              <Skeleton className="h-14 w-full rounded-lg" />
              <Skeleton className="h-14 w-full rounded-lg" />
            </div>
          ) : filteredRefunds.length === 0 ? (
            <Card>
              <CardContent className="py-12">
                <Empty>
                  <EmptyHeader>
                    <EmptyMedia variant="icon">
                      <RotateCcwIcon />
                    </EmptyMedia>
                    <EmptyTitle>当前分组暂无退款单</EmptyTitle>
                    <EmptyDescription>
                      没有符合当前筛选状态的课程退款申请记录。
                    </EmptyDescription>
                  </EmptyHeader>
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
                      <TableHead>退款单号</TableHead>
                      <TableHead>订单号</TableHead>
                      <TableHead>退款金额</TableHead>
                      <TableHead>退款窗口</TableHead>
                      <TableHead>审核状态</TableHead>
                      <TableHead>渠道状态</TableHead>
                      <TableHead>申请时间</TableHead>
                      <TableHead className="text-right">操作</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {filteredRefunds.map((item) => {
                      const appMeta = getRefundApplicationStateMeta(
                        item.applicationState,
                      );
                      const channelMeta = getRefundChannelStateMeta(
                        item.channelState,
                      );
                      return (
                        <TableRow key={item.id}>
                          <TableCell className="font-mono text-xs text-foreground">
                            <div>{item.id}</div>
                            <div className="text-[11px] text-muted-foreground">
                              v{item.version ?? 1}
                            </div>
                          </TableCell>
                          <TableCell className="font-mono text-xs text-muted-foreground">
                            {item.orderId}
                          </TableCell>
                          <TableCell className="font-medium text-foreground whitespace-nowrap">
                            {formatPriceCny(item.amountMinor, false)}
                          </TableCell>
                          <TableCell>
                            <Badge
                              variant={
                                item.isOutOfWindow ? 'default' : 'secondary'
                              }
                            >
                              {item.isOutOfWindow
                                ? '超窗人工申请'
                                : '7×24h 窗口内'}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            <Badge variant={appMeta.variant}>
                              {appMeta.label}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            <Badge variant={channelMeta.variant}>
                              {channelMeta.label}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                            {formatDateTime(item.createdAt)}
                          </TableCell>
                          <TableCell className="text-right">
                            <Button
                              type="button"
                              variant="secondary"
                              size="xs"
                              onClick={() => handleOpenSheet(item)}
                            >
                              <FileSearchIcon />
                              查看与处置
                            </Button>
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </div>

              {/* 移动端卡片 */}
              <div className="grid gap-3 md:hidden">
                {filteredRefunds.map((item) => {
                  const appMeta = getRefundApplicationStateMeta(
                    item.applicationState,
                  );
                  const channelMeta = getRefundChannelStateMeta(
                    item.channelState,
                  );
                  return (
                    <Card key={item.id} size="sm">
                      <CardHeader>
                        <div className="flex items-start justify-between gap-2">
                          <div className="min-w-0 space-y-0.5">
                            <CardTitle className="truncate font-mono text-sm">
                              {item.id}
                            </CardTitle>
                            <CardDescription className="truncate font-mono text-xs">
                              订单：{item.orderId}
                            </CardDescription>
                          </div>
                          <Badge variant={appMeta.variant}>
                            {appMeta.label}
                          </Badge>
                        </div>
                      </CardHeader>
                      <CardContent className="space-y-2 text-xs">
                        <div className="flex flex-wrap gap-1.5">
                          <Badge
                            variant={
                              item.isOutOfWindow ? 'default' : 'secondary'
                            }
                          >
                            {item.isOutOfWindow
                              ? '超窗人工申请'
                              : '7×24h 窗口内'}
                          </Badge>
                          <Badge variant={channelMeta.variant}>
                            渠道：{channelMeta.label}
                          </Badge>
                        </div>
                        <div className="flex items-center justify-between">
                          <span className="text-muted-foreground">
                            退款金额
                          </span>
                          <span className="font-medium text-foreground">
                            {formatPriceCny(item.amountMinor, false)}
                          </span>
                        </div>
                        <div className="flex items-center justify-between">
                          <span className="text-muted-foreground">
                            申请时间
                          </span>
                          <span className="text-muted-foreground">
                            {formatDateTime(item.createdAt)}
                          </span>
                        </div>
                      </CardContent>
                      <CardFooter className="justify-end">
                        <Button
                          type="button"
                          variant="secondary"
                          size="sm"
                          onClick={() => handleOpenSheet(item)}
                        >
                          查看与处置
                        </Button>
                      </CardFooter>
                    </Card>
                  );
                })}
              </div>
            </>
          )}
        </TabsContent>
      </Tabs>

      {/* 退款单详情与处置 Sheet */}
      <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
        <SheetContent side="right" className="overflow-y-auto">
          <SheetHeader>
            <SheetTitle>退款单详情与处置</SheetTitle>
            <SheetDescription>
              核查退款窗口期、异常情形与渠道回执；所有处置均通过乐观锁版本（expectedVersion）校验。
            </SheetDescription>
          </SheetHeader>

          {selectedRefund ? (
            <div className="space-y-5 px-6 py-2 text-sm">
              <div className="flex flex-wrap items-center gap-2">
                <Badge
                  variant={
                    selectedRefund.isOutOfWindow ? 'default' : 'secondary'
                  }
                >
                  {selectedRefund.isOutOfWindow
                    ? '超出 7×24h 窗口（人工审核）'
                    : '7×24h 标准窗口内'}
                </Badge>
                <Badge
                  variant={
                    getRefundApplicationStateMeta(
                      selectedRefund.applicationState,
                    ).variant
                  }
                >
                  审核：
                  {
                    getRefundApplicationStateMeta(
                      selectedRefund.applicationState,
                    ).label
                  }
                </Badge>
                <Badge
                  variant={
                    getRefundChannelStateMeta(selectedRefund.channelState)
                      .variant
                  }
                >
                  渠道：
                  {getRefundChannelStateMeta(selectedRefund.channelState).label}
                </Badge>
              </div>

              <div className="grid gap-3 rounded-lg bg-secondary p-4 text-xs sm:grid-cols-2">
                <div>
                  <div className="text-muted-foreground">退款单号</div>
                  <div className="mt-0.5 font-mono font-medium text-foreground">
                    {selectedRefund.id}（v{selectedRefund.version ?? 1}）
                  </div>
                </div>
                <div>
                  <div className="text-muted-foreground">关联订单号</div>
                  <div className="mt-0.5 font-mono text-foreground">
                    {selectedRefund.orderId}
                  </div>
                </div>
                <div>
                  <div className="text-muted-foreground">申请人用户 ID</div>
                  <div className="mt-0.5 font-mono text-foreground">
                    {selectedRefund.userId || '—'}
                  </div>
                </div>
                <div>
                  <div className="text-muted-foreground">申请退款金额</div>
                  <div className="mt-0.5 text-sm font-medium text-foreground">
                    {formatPriceCny(selectedRefund.amountMinor, false)}
                  </div>
                </div>
                <div>
                  <div className="text-muted-foreground">首次可访问时间</div>
                  <div className="mt-0.5 text-foreground">
                    {selectedRefund.firstAvailableAt
                      ? formatDateTime(selectedRefund.firstAvailableAt)
                      : '—'}
                  </div>
                </div>
                <div>
                  <div className="text-muted-foreground">
                    7×24h 窗口截止时间
                  </div>
                  <div className="mt-0.5 text-foreground">
                    {selectedRefund.deadline
                      ? formatDateTime(selectedRefund.deadline)
                      : '—'}
                  </div>
                </div>
                <div>
                  <div className="text-muted-foreground">申请提交时间</div>
                  <div className="mt-0.5 text-foreground">
                    {formatDateTime(selectedRefund.createdAt)}
                  </div>
                </div>
                <div>
                  <div className="text-muted-foreground">最近审核时间</div>
                  <div className="mt-0.5 text-foreground">
                    {selectedRefund.reviewedAt
                      ? formatDateTime(selectedRefund.reviewedAt)
                      : '尚未审核'}
                  </div>
                </div>
              </div>

              <div className="space-y-3 rounded-lg bg-secondary p-4 text-xs">
                <div>
                  <div className="text-muted-foreground">申请情形类型</div>
                  <div className="mt-0.5 font-medium text-foreground">
                    {getRefundExceptionLabel(selectedRefund.exceptionType)}
                  </div>
                </div>
                <div>
                  <div className="text-muted-foreground">申请人填写原因</div>
                  <div className="mt-0.5 leading-relaxed text-foreground">
                    {selectedRefund.reason || '未填写'}
                  </div>
                </div>
                {selectedRefund.rejectionReason ? (
                  <div className="rounded-md bg-destructive-surface p-2.5 text-foreground">
                    审核驳回原因：{selectedRefund.rejectionReason}
                  </div>
                ) : null}
                {selectedRefund.channelRefundId ? (
                  <div>
                    <span className="text-muted-foreground">
                      渠道退款流水号：
                    </span>
                    <span className="font-mono text-foreground">
                      {selectedRefund.channelRefundId}
                    </span>
                  </div>
                ) : null}
                {selectedRefund.channelFailureReason ? (
                  <div className="rounded-md bg-destructive-surface p-2.5 text-foreground">
                    渠道执行异常原因：{selectedRefund.channelFailureReason}
                  </div>
                ) : null}
              </div>

              {/* 待审核处置区 */}
              {isPendingReviewRefund(selectedRefund) ? (
                <div className="space-y-3 rounded-lg bg-secondary p-4">
                  <div className="text-xs font-medium text-foreground">
                    第一步：管理员审核申请（pending_review）
                  </div>
                  <p className="text-xs leading-relaxed text-muted-foreground">
                    批准后退款单将进入“已批准 /
                    渠道提交中（submitting）”，此时学员课程学习权益仍保持有效，直至渠道退款确认成功。
                  </p>
                  <div className="flex flex-wrap gap-2">
                    <Button
                      type="button"
                      size="sm"
                      onClick={() =>
                        openReviewDialog(selectedRefund, 'approved')
                      }
                    >
                      <CheckCircle2Icon />
                      批准并提交原路退款
                    </Button>
                    <Button
                      type="button"
                      variant="destructive"
                      size="sm"
                      onClick={() =>
                        openReviewDialog(selectedRefund, 'rejected')
                      }
                    >
                      <XCircleIcon />
                      拒绝退款
                    </Button>
                  </div>
                </div>
              ) : null}

              {/* 已批准待渠道确认处置区 */}
              {isChannelProcessingRefund(selectedRefund) ? (
                <div className="space-y-3 rounded-lg bg-secondary p-4">
                  <div className="text-xs font-medium text-foreground">
                    第二步：登记渠道原路退款结果（channelState=
                    {selectedRefund.channelState}）
                  </div>
                  <p className="text-xs leading-relaxed text-muted-foreground">
                    请根据支付渠道回调或对账单登记渠道退款结果。仅当登记为
                    <span className="font-mono text-foreground"> success </span>
                    时，系统才会将该笔订单标记为“已退款”并撤销对应学习权益；若为
                    <span className="font-mono text-foreground"> failed </span>
                    或
                    <span className="font-mono text-foreground"> unknown</span>
                    ，学习权益继续保留并可再次核实登记。
                  </p>
                  <div>
                    <Button
                      type="button"
                      size="sm"
                      onClick={() => openChannelConfirmDialog(selectedRefund)}
                    >
                      登记渠道退款结果
                    </Button>
                  </div>
                </div>
              ) : null}
            </div>
          ) : null}

          <SheetFooter>
            <Button
              type="button"
              variant="secondary"
              onClick={() => setSheetOpen(false)}
            >
              关闭面板
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      {/* 批准 / 拒绝退款 AlertDialog */}
      <AlertDialog
        open={Boolean(reviewTarget)}
        onOpenChange={(open) => {
          if (!open && !submittingReview) {
            setReviewTarget(null);
            setReviewReason('');
            setReviewReasonError(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {reviewTarget?.decision === 'approved'
                ? '确认批准并提交原路退款'
                : '确认拒绝该笔退款申请'}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {reviewTarget?.decision === 'approved'
                ? `批准退款单（${reviewTarget.refund.id}，金额 ${formatPriceCny(reviewTarget.refund.amountMinor, false)}）后，将转入渠道原路退款阶段（channelState=submitting）。在渠道确认成功前，学员学习权益继续保持有效。`
                : `拒绝退款单（${reviewTarget?.refund.id}）后，该申请将进入已驳回终态，学员可查看您填写的驳回说明。`}
            </AlertDialogDescription>
          </AlertDialogHeader>

          <FieldGroup>
            <Field data-invalid={Boolean(reviewReasonError)}>
              <FieldLabel htmlFor="review-refund-reason">
                {reviewTarget?.decision === 'approved'
                  ? '审核备注（选填）'
                  : '拒绝原因（必填）'}
              </FieldLabel>
              <Textarea
                id="review-refund-reason"
                value={reviewReason}
                onChange={(e) => setReviewReason(e.target.value)}
                placeholder={
                  reviewTarget?.decision === 'approved'
                    ? '可填写核准依据或渠道提交备注'
                    : '请明确说明不符合退款条件的具体原因'
                }
                aria-invalid={Boolean(reviewReasonError)}
              />
              <FieldDescription>
                乐观锁版本：expectedVersion=v
                {reviewTarget?.refund.version ?? 1}
              </FieldDescription>
              <FieldError>{reviewReasonError}</FieldError>
            </Field>
          </FieldGroup>

          <AlertDialogFooter>
            <AlertDialogCancel disabled={submittingReview}>
              取消
            </AlertDialogCancel>
            <AlertDialogAction
              variant={
                reviewTarget?.decision === 'rejected'
                  ? 'destructive'
                  : 'default'
              }
              disabled={submittingReview}
              onClick={(e) => void handleConfirmReview(e)}
            >
              {submittingReview ? <Spinner /> : null}
              {submittingReview
                ? '提交中…'
                : reviewTarget?.decision === 'approved'
                  ? '确认批准退款'
                  : '确认拒绝退款'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* 登记渠道退款结果 Dialog */}
      <Dialog
        open={channelDialogOpen}
        onOpenChange={(open) => {
          if (!open && !submittingChannel) {
            setChannelDialogOpen(false);
            setChannelTarget(null);
            setChannelFormError(null);
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>登记渠道退款结果</DialogTitle>
            <DialogDescription>
              更新退款单（{channelTarget?.id}
              ）的渠道执行状态（channelState）。请注意：仅当选择
              <span className="font-mono text-foreground"> success </span>
              时，系统才会撤销该笔订单对应的课程学习权益并将订单标记为已退款。
            </DialogDescription>
          </DialogHeader>

          <form
            onSubmit={(e) => void handleConfirmChannelSubmit(e)}
            noValidate
            className="space-y-4"
          >
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="channel-status-select">
                  渠道退款执行结果（status）
                </FieldLabel>
                <Select
                  value={channelStatus}
                  onValueChange={(v) =>
                    setChannelStatus(v as 'success' | 'failed' | 'unknown')
                  }
                >
                  <SelectTrigger id="channel-status-select">
                    <SelectValue placeholder="选择渠道执行状态" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="success">
                      success — 原路退款成功（立即撤销该笔订单学习权益）
                    </SelectItem>
                    <SelectItem value="failed">
                      failed — 渠道退款失败（保留学习权益，待人工重发或核查）
                    </SelectItem>
                    <SelectItem value="unknown">
                      unknown — 渠道状态未知/超时（保留学习权益，待对账确认）
                    </SelectItem>
                  </SelectContent>
                </Select>
                <FieldDescription>
                  仅 <span className="font-mono">success</span>{' '}
                  会触发权益撤销与订单退款终态。
                </FieldDescription>
              </Field>

              <Field>
                <FieldLabel htmlFor="channel-refund-id-input">
                  渠道退款单号（channelRefundId）
                </FieldLabel>
                <Input
                  id="channel-refund-id-input"
                  value={channelRefundId}
                  onChange={(e) => setChannelRefundId(e.target.value)}
                  placeholder="如支付网关返回的退款流水号"
                />
              </Field>

              <Field data-invalid={Boolean(channelFormError)}>
                <FieldLabel htmlFor="channel-failure-reason">
                  渠道失败或异常说明（failureReason）
                </FieldLabel>
                <Textarea
                  id="channel-failure-reason"
                  value={channelFailureReason}
                  onChange={(e) => setChannelFailureReason(e.target.value)}
                  placeholder="当状态为 failed 或 unknown 时必填，记录网关错误码或待对账原因"
                  aria-invalid={Boolean(channelFormError)}
                />
                <FieldDescription>
                  乐观锁版本：expectedVersion=v{channelTarget?.version ?? 1}
                </FieldDescription>
                <FieldError>{channelFormError}</FieldError>
              </Field>
            </FieldGroup>

            <DialogFooter>
              <Button
                type="button"
                variant="secondary"
                disabled={submittingChannel}
                onClick={() => setChannelDialogOpen(false)}
              >
                取消
              </Button>
              <Button type="submit" disabled={submittingChannel}>
                {submittingChannel ? <Spinner /> : null}
                {submittingChannel ? '提交中…' : '确认登记渠道结果'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}
