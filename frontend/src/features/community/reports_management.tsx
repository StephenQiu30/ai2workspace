'use client';

import {useMemo, useState} from 'react';
import Link from 'next/link';
import {
  AlertCircleIcon,
  ArrowUpRightIcon,
  CheckCircle2Icon,
  EyeIcon,
  EyeOffIcon,
  FlagIcon,
  RefreshCwIcon,
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
import {Card, CardContent} from '@/components/ui/card';
import {
  Empty,
  EmptyContent,
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
import {Separator} from '@/components/ui/separator';
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
import {formatDateTime, getErrorMessage} from '@/lib/format';
import {
  communityListReports,
  communityModerateReport,
} from '@/services/generated/community/community';

export interface ReportsManagementProps {
  initialReports: CommunityApi.Report[];
  initialTotal: number;
  initialError?: string | null;
}

type ReportTabKey = 'pending' | 'processed' | 'all';

interface ModerationActionState {
  report: CommunityApi.Report;
  decision: 'approved' | 'rejected';
}

const REPORTS_PAGE_SIZE = 50;

function getReportStatusLabel(status?: CommunityApi.ReportStatus): string {
  switch (status) {
    case 'pending':
      return '待处理';
    case 'approved':
      return '已隐藏内容';
    case 'rejected':
      return '已驳回';
    default:
      return status || '未知';
  }
}

function getTargetTypeLabel(targetType?: string): string {
  switch (targetType) {
    case 'post':
      return '讨论帖';
    case 'comment':
      return '评论回复';
    default:
      return targetType || '未知对象';
  }
}

export function ReportsManagement({
  initialReports,
  initialTotal,
  initialError = null,
}: ReportsManagementProps) {
  const [activeTab, setActiveTab] = useState<ReportTabKey>('pending');
  const [reports, setReports] = useState<CommunityApi.Report[]>(initialReports);
  const [totalCount, setTotalCount] = useState<number>(initialTotal);
  const [loading, setLoading] = useState(false);
  const [errorBanner, setErrorBanner] = useState<string | null>(initialError);

  // Sheet 查看举报详情
  const [selectedReport, setSelectedReport] =
    useState<CommunityApi.Report | null>(null);

  // AlertDialog 执行审核处置
  const [moderationAction, setModerationAction] =
    useState<ModerationActionState | null>(null);
  const [reviewReason, setReviewReason] = useState('');
  const [reviewError, setReviewError] = useState<string | null>(null);
  const [moderating, setModerating] = useState(false);

  const pendingReports = useMemo(
    () => reports.filter((item) => (item.status ?? 'pending') === 'pending'),
    [reports],
  );
  const processedReports = useMemo(
    () => reports.filter((item) => item.status && item.status !== 'pending'),
    [reports],
  );

  async function refreshReports() {
    if (loading) {
      return;
    }
    setLoading(true);
    setErrorBanner(null);
    try {
      const resp = await communityListReports({
        page: 1,
        pageSize: REPORTS_PAGE_SIZE,
      });
      const items = resp.items ?? [];
      setReports(items);
      setTotalCount(resp.total ?? items.length);
      if (selectedReport?.id) {
        const updated = items.find((r) => r.id === selectedReport.id);
        if (updated) {
          setSelectedReport(updated);
        }
      }
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '刷新举报队列失败');
      setErrorBanner(msg);
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  }

  function openModerationDialog(
    report: CommunityApi.Report,
    decision: 'approved' | 'rejected',
  ) {
    setModerationAction({report, decision});
    setReviewReason('');
    setReviewError(null);
  }

  async function handleConfirmModeration() {
    const reportId = moderationAction?.report.id;
    if (!reportId || !moderationAction || moderating) {
      return;
    }

    const cleanReason = reviewReason.trim();
    if (!cleanReason) {
      setReviewError(
        moderationAction.decision === 'approved'
          ? '请填写隐藏该内容的处置说明（将记录审计日志）'
          : '请填写驳回该举报的核查说明',
      );
      return;
    }

    setReviewError(null);
    setModerating(true);

    try {
      const updated = await communityModerateReport(
        {id: reportId},
        {
          decision: moderationAction.decision,
          reason: cleanReason,
        },
      );

      setReports((prev) =>
        prev.map((item) =>
          item.id === reportId
            ? {
                ...item,
                ...updated,
                status: updated.status ?? moderationAction.decision,
                reviewReason: updated.reviewReason ?? cleanReason,
              }
            : item,
        ),
      );
      if (selectedReport?.id === reportId) {
        setSelectedReport((prev) =>
          prev
            ? {
                ...prev,
                ...updated,
                status: updated.status ?? moderationAction.decision,
                reviewReason: updated.reviewReason ?? cleanReason,
              }
            : null,
        );
      }

      toast.success(
        moderationAction.decision === 'approved'
          ? '已通过举报并隐藏目标内容'
          : '已驳回该条举报',
      );
      setModerationAction(null);
      setReviewReason('');
    } catch (err: unknown) {
      const msg = getErrorMessage(err, '处置举报失败，请稍后重试');
      setReviewError(msg);
      toast.error(msg);
    } finally {
      setModerating(false);
    }
  }

  function renderTableForList(
    list: CommunityApi.Report[],
    tabKey: ReportTabKey,
  ) {
    if (loading) {
      return (
        <div className="flex flex-col gap-2.5">
          <Skeleton className="h-12 w-full rounded-lg" />
          <Skeleton className="h-14 w-full rounded-lg" />
          <Skeleton className="h-14 w-full rounded-lg" />
        </div>
      );
    }

    if (list.length === 0) {
      return (
        <Card>
          <CardContent className="py-12">
            <Empty>
              <EmptyHeader>
                <EmptyMedia variant="icon">
                  <FlagIcon />
                </EmptyMedia>
                <EmptyTitle>
                  {tabKey === 'pending'
                    ? '当前没有待处理的内容举报'
                    : tabKey === 'processed'
                      ? '暂无已处理的举报记录'
                      : '暂无任何举报记录'}
                </EmptyTitle>
                <EmptyDescription>
                  {tabKey === 'pending'
                    ? '所有社区举报均已审核完毕，可切换到「已处理」或「全部」查阅历史审计记录。'
                    : '用户提交的讨论帖或评论违规举报将会汇总展示在此队列中。'}
                </EmptyDescription>
              </EmptyHeader>
              {tabKey !== 'all' && reports.length > 0 ? (
                <EmptyContent>
                  <Button
                    type="button"
                    variant="secondary"
                    size="sm"
                    onClick={() => setActiveTab('all')}
                  >
                    查看全部举报记录
                  </Button>
                </EmptyContent>
              ) : null}
            </Empty>
          </CardContent>
        </Card>
      );
    }

    return (
      <Card className="py-2">
        <CardContent className="px-4">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>举报编号</TableHead>
                <TableHead>目标类型</TableHead>
                <TableHead>目标 ID</TableHead>
                <TableHead>举报原因</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>提交时间</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {list.map((report) => {
                const isPending = (report.status ?? 'pending') === 'pending';
                return (
                  <TableRow key={report.id}>
                    <TableCell className="font-mono text-xs text-foreground">
                      {report.id || '-'}
                    </TableCell>
                    <TableCell>
                      <Badge variant="secondary">
                        {getTargetTypeLabel(report.targetType)}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {report.targetType === 'post' && report.targetId ? (
                        <Link
                          href={`/community/${report.targetId}`}
                          className="inline-flex items-center gap-1 text-foreground underline underline-offset-4 hover:opacity-80"
                        >
                          {report.targetId}
                          <ArrowUpRightIcon className="size-3" />
                        </Link>
                      ) : (
                        report.targetId || '-'
                      )}
                    </TableCell>
                    <TableCell className="max-w-xs truncate text-sm text-body">
                      {report.reason || '-'}
                    </TableCell>
                    <TableCell>
                      <Badge variant={isPending ? 'default' : 'secondary'}>
                        {getReportStatusLabel(report.status)}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {formatDateTime(report.createdAt)}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex flex-wrap items-center justify-end gap-1.5">
                        <Button
                          type="button"
                          variant="secondary"
                          size="xs"
                          onClick={() => setSelectedReport(report)}
                        >
                          <EyeIcon data-icon="inline-start" />
                          查看详情
                        </Button>
                        {isPending ? (
                          <>
                            <Button
                              type="button"
                              variant="secondary"
                              size="xs"
                              onClick={() =>
                                openModerationDialog(report, 'approved')
                              }
                            >
                              <EyeOffIcon data-icon="inline-start" />
                              隐藏内容
                            </Button>
                            <Button
                              type="button"
                              variant="ghost"
                              size="xs"
                              onClick={() =>
                                openModerationDialog(report, 'rejected')
                              }
                            >
                              <XCircleIcon data-icon="inline-start" />
                              驳回举报
                            </Button>
                          </>
                        ) : null}
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-col gap-1.5">
          <h1 className="font-heading text-[28px] leading-[40px] font-medium text-foreground">
            内容举报治理
          </h1>
          <p className="text-sm leading-6 text-muted-foreground">
            审核社区讨论帖与评论回复的违规举报；确认违规后将立即隐藏对应内容并发送治理通知（不在通知中泄露被隐藏原文，共{' '}
            {totalCount} 条记录）。
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            disabled={loading}
            onClick={() => void refreshReports()}
          >
            <RefreshCwIcon
              data-icon="inline-start"
              className={loading ? 'animate-spin' : undefined}
            />
            刷新队列
          </Button>
        </div>
      </header>

      {errorBanner ? (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>举报数据加载异常</AlertTitle>
          <AlertDescription>{errorBanner}</AlertDescription>
        </Alert>
      ) : null}

      <Tabs
        value={activeTab}
        onValueChange={(val) => setActiveTab(val as ReportTabKey)}
        className="flex flex-col gap-5"
      >
        <TabsList>
          <TabsTrigger value="pending">
            待处理
            <Badge
              variant="secondary"
              className="ml-1.5 h-5 px-1.5 text-[11px]"
            >
              {pendingReports.length}
            </Badge>
          </TabsTrigger>
          <TabsTrigger value="processed">
            已处理
            <Badge
              variant="secondary"
              className="ml-1.5 h-5 px-1.5 text-[11px]"
            >
              {processedReports.length}
            </Badge>
          </TabsTrigger>
          <TabsTrigger value="all">
            全部
            <Badge
              variant="secondary"
              className="ml-1.5 h-5 px-1.5 text-[11px]"
            >
              {reports.length}
            </Badge>
          </TabsTrigger>
        </TabsList>

        <TabsContent value="pending">
          {renderTableForList(pendingReports, 'pending')}
        </TabsContent>

        <TabsContent value="processed">
          {renderTableForList(processedReports, 'processed')}
        </TabsContent>

        <TabsContent value="all">
          {renderTableForList(reports, 'all')}
        </TabsContent>
      </Tabs>

      {/* 举报详情 Sheet */}
      <Sheet
        open={Boolean(selectedReport)}
        onOpenChange={(open) => {
          if (!open) {
            setSelectedReport(null);
          }
        }}
      >
        <SheetContent
          side="right"
          className="w-full overflow-y-auto sm:max-w-lg"
        >
          <SheetHeader>
            <SheetTitle>举报记录详情</SheetTitle>
            <SheetDescription>
              核对举报人提交的违规线索、目标对象信息以及历史处置结论。
            </SheetDescription>
          </SheetHeader>

          {selectedReport ? (
            <div className="flex flex-1 flex-col gap-5 px-6 pb-6">
              <div className="flex flex-wrap items-center gap-2">
                <Badge
                  variant={
                    (selectedReport.status ?? 'pending') === 'pending'
                      ? 'default'
                      : 'secondary'
                  }
                >
                  {getReportStatusLabel(selectedReport.status)}
                </Badge>
                <Badge variant="secondary">
                  {getTargetTypeLabel(selectedReport.targetType)}
                </Badge>
              </div>

              <div className="flex flex-col gap-3 rounded-xl bg-background p-4 text-sm">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-xs text-muted-foreground">
                    举报编号
                  </span>
                  <span className="font-mono text-xs text-foreground">
                    {selectedReport.id || '-'}
                  </span>
                </div>
                <Separator />
                <div className="flex items-center justify-between gap-2">
                  <span className="text-xs text-muted-foreground">
                    目标对象 ID
                  </span>
                  <span className="font-mono text-xs text-foreground">
                    {selectedReport.targetId || '-'}
                  </span>
                </div>
                <Separator />
                <div className="flex items-center justify-between gap-2">
                  <span className="text-xs text-muted-foreground">
                    举报人 ID
                  </span>
                  <span className="font-mono text-xs text-foreground">
                    {selectedReport.reporterId || '-'}
                  </span>
                </div>
                <Separator />
                <div className="flex items-center justify-between gap-2">
                  <span className="text-xs text-muted-foreground">
                    提交时间
                  </span>
                  <span className="text-xs text-foreground">
                    {formatDateTime(selectedReport.createdAt)}
                  </span>
                </div>
              </div>

              <div className="flex flex-col gap-1.5">
                <span className="text-xs leading-5 font-medium text-muted-foreground">
                  举报人填写的违规原因
                </span>
                <div className="rounded-xl bg-background p-4 text-sm leading-6 text-body">
                  {selectedReport.reason || '未提供详细说明'}
                </div>
              </div>

              {selectedReport.targetType === 'post' &&
              selectedReport.targetId ? (
                <div>
                  <Button variant="secondary" size="sm" asChild>
                    <Link href={`/community/${selectedReport.targetId}`}>
                      前往目标讨论页核查
                      <ArrowUpRightIcon data-icon="inline-end" />
                    </Link>
                  </Button>
                </div>
              ) : null}

              {selectedReport.status && selectedReport.status !== 'pending' ? (
                <div className="flex flex-col gap-2 rounded-xl bg-background p-4">
                  <div className="flex items-center gap-1.5 text-xs font-medium text-foreground">
                    <CheckCircle2Icon className="size-3.5" />
                    处置审计记录
                  </div>
                  <p className="text-xs leading-5 text-muted-foreground">
                    处理人：{selectedReport.reviewerId || '管理员'} · 处理时间：
                    {formatDateTime(selectedReport.reviewedAt)}
                  </p>
                  <p className="text-sm leading-6 text-body">
                    结论说明：{selectedReport.reviewReason || '无附加说明'}
                  </p>
                </div>
              ) : null}

              <SheetFooter className="mt-auto flex-row justify-end gap-2 px-0 pb-0">
                {(selectedReport.status ?? 'pending') === 'pending' ? (
                  <>
                    <Button
                      type="button"
                      variant="secondary"
                      onClick={() =>
                        openModerationDialog(selectedReport, 'rejected')
                      }
                    >
                      <XCircleIcon data-icon="inline-start" />
                      驳回举报
                    </Button>
                    <Button
                      type="button"
                      onClick={() =>
                        openModerationDialog(selectedReport, 'approved')
                      }
                    >
                      <EyeOffIcon data-icon="inline-start" />
                      隐藏内容
                    </Button>
                  </>
                ) : (
                  <Button
                    type="button"
                    variant="secondary"
                    onClick={() => setSelectedReport(null)}
                  >
                    关闭
                  </Button>
                )}
              </SheetFooter>
            </div>
          ) : null}
        </SheetContent>
      </Sheet>

      {/* 审核确认 AlertDialog（隐藏内容 approved / 驳回举报 rejected） */}
      <AlertDialog
        open={Boolean(moderationAction)}
        onOpenChange={(open) => {
          if (!open && !moderating) {
            setModerationAction(null);
            setReviewError(null);
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {moderationAction?.decision === 'approved'
                ? '确认隐藏该举报目标内容？'
                : '确认驳回该条内容举报？'}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {moderationAction?.decision === 'approved'
                ? '执行「隐藏内容 (approved)」后，被举报的讨论帖或评论将立即停止公开展示，直链返回管理员已隐藏说明，且系统通知不会泄露被隐藏正文。'
                : '执行「驳回举报 (rejected)」后，目标内容将保持正常展示，并向举报人反馈核查结论。'}
            </AlertDialogDescription>
          </AlertDialogHeader>

          <FieldGroup>
            <Field data-invalid={Boolean(reviewError)}>
              <FieldLabel htmlFor="moderate-report-reason">
                处置结论说明
              </FieldLabel>
              <Textarea
                id="moderate-report-reason"
                value={reviewReason}
                rows={3}
                maxLength={300}
                disabled={moderating}
                placeholder={
                  moderationAction?.decision === 'approved'
                    ? '例如：经核查包含违规引流内容，依社区规范予以隐藏…'
                    : '例如：经核查属于正常技术讨论，未构成违规…'
                }
                aria-invalid={Boolean(reviewError)}
                onChange={(e) => {
                  setReviewReason(e.target.value);
                  setReviewError(null);
                }}
              />
              <FieldDescription>
                该说明将留存治理审计记录，并用于生成脱敏的站内处理通知。
              </FieldDescription>
              {reviewError ? <FieldError>{reviewError}</FieldError> : null}
            </Field>
          </FieldGroup>

          <AlertDialogFooter>
            <AlertDialogCancel disabled={moderating}>取消</AlertDialogCancel>
            <AlertDialogAction
              variant={
                moderationAction?.decision === 'approved'
                  ? 'destructive'
                  : 'default'
              }
              disabled={moderating}
              onClick={(e) => {
                e.preventDefault();
                void handleConfirmModeration();
              }}
            >
              {moderating ? <Spinner data-icon="inline-start" /> : null}
              {moderationAction?.decision === 'approved'
                ? '确认隐藏内容'
                : '确认驳回举报'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
