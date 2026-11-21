'use client';

import {useState} from 'react';
import {
  AlertCircleIcon,
  CheckCircle2Icon,
  CpuIcon,
  PauseCircleIcon,
  PencilIcon,
  PlayCircleIcon,
  ReceiptTextIcon,
  SlidersHorizontalIcon,
} from 'lucide-react';
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
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Checkbox} from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
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
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field';
import {Input} from '@/components/ui/input';
import {Progress} from '@/components/ui/progress';
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
import {
  computeUsagePercentage,
  formatCentsToYuan,
  formatPricePerK,
  getProviderLabel,
  getPurposeLabel,
  getReconciliationStateLabel,
  isModelExpired,
} from '@/features/assistant/assistant_helpers';
import {formatDateTime, getErrorMessage} from '@/lib/format';
import {
  assistantGetGovernanceSummary,
  assistantListModels,
  assistantListReconciliations,
  assistantResolveReconciliation,
  assistantSetAiPause,
  assistantUpdateBudgetConfig,
  assistantUpdateModel,
} from '@/services/generated/assistant/governance';

export interface AiUsageViewProps {
  initialSummary: AssistantApi.UsageSummary | null;
  initialModels: AssistantApi.ModelInfo[];
  initialReconciliations: AssistantApi.ReconciliationItem[];
  initialError?: string | null;
}

export function AiUsageView({
  initialSummary,
  initialModels,
  initialReconciliations,
  initialError,
}: AiUsageViewProps) {
  const [summary, setSummary] = useState<AssistantApi.UsageSummary | null>(
    initialSummary,
  );
  const [models, setModels] = useState<AssistantApi.ModelInfo[]>(initialModels);
  const [reconciliations, setReconciliations] = useState<
    AssistantApi.ReconciliationItem[]
  >(initialReconciliations);
  const [errorMsg, setErrorMsg] = useState<string | null>(initialError ?? null);
  const [statusMsg, setStatusMsg] = useState<string | null>(null);

  // Pause / Resume AI Request Dialog
  const [pauseReason, setPauseReason] = useState(
    initialSummary?.pauseReason || '管理员手动临时暂停新增 AI 请求',
  );
  const [togglingPause, setTogglingPause] = useState(false);

  // Adjust Budget Config Dialog
  const [budgetOpen, setBudgetOpen] = useState(false);
  const [platformDailyCents, setPlatformDailyCents] = useState(
    String(initialSummary?.platformDailyLimitCents ?? 10000),
  );
  const [platformMonthlyCents, setPlatformMonthlyCents] = useState(
    String(initialSummary?.platformMonthlyLimitCents ?? 200000),
  );
  const [backgroundDailyCents, setBackgroundDailyCents] = useState(
    String(initialSummary?.backgroundDailyLimitCents ?? 2000),
  );
  const [userDailyCents, setUserDailyCents] = useState(
    String(initialSummary?.maxDailyCostCents ?? 200),
  );
  const [userDailyQuestions, setUserDailyQuestions] = useState(
    String(initialSummary?.maxDailyQuestions ?? 20),
  );
  const [alertThresholdPct, setAlertThresholdPct] = useState('80');
  const [savingBudget, setSavingBudget] = useState(false);

  // Edit Model Dialog
  const [editingModel, setEditingModel] =
    useState<AssistantApi.ModelInfo | null>(null);
  const [modelEnabled, setModelEnabled] = useState(true);
  const [modelDisplayName, setModelDisplayName] = useState('');
  const [modelPromptPrice, setModelPromptPrice] = useState('1');
  const [modelCompletionPrice, setModelCompletionPrice] = useState('2');
  const [modelPriceVersion, setModelPriceVersion] = useState('v1');
  const [modelExpiresAt, setModelExpiresAt] = useState('');
  const [savingModel, setSavingModel] = useState(false);

  // Resolve Reconciliation Dialog
  const [resolvingItem, setResolvingItem] =
    useState<AssistantApi.ReconciliationItem | null>(null);
  const [resolvedCentsInput, setResolvedCentsInput] = useState('0');
  const [resolutionNote, setResolutionNote] = useState('');
  const [resolving, setResolving] = useState(false);

  const refreshAll = async () => {
    try {
      const [sumRes, modRes, recRes] = await Promise.all([
        assistantGetGovernanceSummary(),
        assistantListModels(),
        assistantListReconciliations(),
      ]);
      if (sumRes) {
        setSummary(sumRes);
      }
      if (Array.isArray(modRes)) {
        setModels(modRes);
      }
      if (Array.isArray(recRes)) {
        setReconciliations(recRes);
      }
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '刷新 AI 治理数据失败'));
    }
  };

  const handleOpenBudgetDialog = () => {
    setPlatformDailyCents(String(summary?.platformDailyLimitCents ?? 10000));
    setPlatformMonthlyCents(
      String(summary?.platformMonthlyLimitCents ?? 200000),
    );
    setBackgroundDailyCents(String(summary?.backgroundDailyLimitCents ?? 2000));
    setUserDailyCents(String(summary?.maxDailyCostCents ?? 200));
    setUserDailyQuestions(String(summary?.maxDailyQuestions ?? 20));
    setBudgetOpen(true);
  };

  const handleTogglePause = async () => {
    const nextPaused = !summary?.paused;
    setTogglingPause(true);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      await assistantSetAiPause({
        paused: nextPaused,
        reason: nextPaused
          ? pauseReason.trim() || '管理员手动暂停新增 AI 请求'
          : '',
      });
      setStatusMsg(
        nextPaused
          ? '已暂停全平台新增 AI 请求准入（问答与入库作业将被阻断）。'
          : '已恢复全平台 AI 请求准入。',
      );
      await refreshAll();
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '切换平台暂停状态失败'));
    } finally {
      setTogglingPause(false);
    }
  };

  const handleSaveBudget = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setSavingBudget(true);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      await assistantUpdateBudgetConfig({
        platformDailyCents: Number(platformDailyCents) || 10000,
        platformMonthlyCents: Number(platformMonthlyCents) || 200000,
        backgroundDailyCents: Number(backgroundDailyCents) || 2000,
        userDailyCents: Number(userDailyCents) || 200,
        userDailyQuestions: Number(userDailyQuestions) || 20,
        alertThresholdPct: Number(alertThresholdPct) || 80,
      });
      setBudgetOpen(false);
      setStatusMsg('平台与账号 AI 预算限额及 80% 预警阈值已更新生效。');
      await refreshAll();
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '更新预算配置失败'));
    } finally {
      setSavingBudget(false);
    }
  };

  const handleOpenEditModel = (m: AssistantApi.ModelInfo) => {
    setEditingModel(m);
    setModelEnabled(m.enabled !== false);
    setModelDisplayName(m.displayName ?? '');
    setModelPromptPrice(String(m.promptPricePerK ?? 1));
    setModelCompletionPrice(String(m.completionPricePerK ?? 2));
    setModelPriceVersion(m.priceVersion ?? 'v1');
    setModelExpiresAt(m.expiresAt ?? '');
    setErrorMsg(null);
  };

  const handleSaveModel = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!editingModel?.providerId || !editingModel?.modelId) {
      return;
    }

    setSavingModel(true);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      await assistantUpdateModel(
        {
          providerId: editingModel.providerId,
          modelId: editingModel.modelId,
        },
        {
          enabled: modelEnabled,
          displayName: modelDisplayName.trim() || editingModel.displayName,
          promptPricePerK: Number(modelPromptPrice),
          completionPricePerK: Number(modelCompletionPrice),
          priceVersion: modelPriceVersion.trim() || 'v1',
          expiresAt: modelExpiresAt.trim() || undefined,
        },
      );
      setEditingModel(null);
      setStatusMsg(
        `模型「${editingModel.providerId}:${editingModel.modelId}」启停状态与价目版本已更新。`,
      );
      await refreshAll();
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '更新模型价目与状态失败'));
    } finally {
      setSavingModel(false);
    }
  };

  const handleOpenResolve = (item: AssistantApi.ReconciliationItem) => {
    setResolvingItem(item);
    setResolvedCentsInput(String(item.unknownCents ?? 0));
    setResolutionNote(item.resolutionNote ?? '');
    setErrorMsg(null);
  };

  const handleResolveReconciliation = async (
    e: React.FormEvent<HTMLFormElement>,
  ) => {
    e.preventDefault();
    if (!resolvingItem?.id) {
      return;
    }

    setResolving(true);
    setErrorMsg(null);
    setStatusMsg(null);
    try {
      await assistantResolveReconciliation(
        {id: resolvingItem.id},
        {
          resolvedCents: Number(resolvedCentsInput) || 0,
          note: resolutionNote.trim() || '管理员人工核对结清',
        },
      );
      setResolvingItem(null);
      setStatusMsg(
        `未知费用记录 #${resolvingItem.id} 已完成人工结算核对，预占冻结金额已按实际核定金额结清。`,
      );
      await refreshAll();
    } catch (err: unknown) {
      setErrorMsg(getErrorMessage(err, '人工结算核对失败'));
    } finally {
      setResolving(false);
    }
  };

  // Compute real governance numbers from summary
  const dailyConfirmed = summary?.platformCostCents ?? 0;
  const dailyReserved = summary?.platformDailyReservedCents ?? 0;
  const dailyUnknown = summary?.platformUnknownCents ?? 0;
  const dailyUsedTotal = dailyConfirmed + dailyReserved + dailyUnknown;
  const dailyLimit = summary?.platformDailyLimitCents ?? 0;
  const dailyPct = computeUsagePercentage(dailyUsedTotal, dailyLimit);

  const monthlyConfirmed = summary?.platformMonthlyCostCents ?? 0;
  const monthlyReserved = summary?.platformMonthlyReservedCents ?? 0;
  const monthlyUsedTotal = monthlyConfirmed + monthlyReserved + dailyUnknown;
  const monthlyLimit = summary?.platformMonthlyLimitCents ?? 0;
  const monthlyPct = computeUsagePercentage(monthlyUsedTotal, monthlyLimit);

  const bgConfirmed = summary?.backgroundDailyCostCents ?? 0;
  const bgReserved = summary?.backgroundDailyReservedCents ?? 0;
  const bgUsedTotal = bgConfirmed + bgReserved;
  const bgLimit = summary?.backgroundDailyLimitCents ?? 0;
  const bgPct = computeUsagePercentage(bgUsedTotal, bgLimit);

  const personalUsedCents =
    (summary?.totalCostCents ?? 0) +
    (summary?.reservedCostCents ?? 0) +
    (summary?.unknownCostCents ?? 0);
  const personalLimitCents = summary?.maxDailyCostCents ?? 0;

  return (
    <div className="space-y-6">
      {/* Top Global Status & Actions */}
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-start">
        <div className="space-y-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-semibold tracking-tight">
              四家模型目录与 AI 费用治理
            </h1>
            <Badge variant={summary?.paused ? 'destructive' : 'default'}>
              {summary?.paused ? '已暂停新增 AI 请求' : '平台准入运行正常'}
            </Badge>
            {summary?.dailyAlertTriggered ? (
              <Badge variant="destructive">日预算 80% 预警触发</Badge>
            ) : null}
            {summary?.monthlyAlertTriggered ? (
              <Badge variant="destructive">月预算 80% 预警触发</Badge>
            ) : null}
          </div>
          <p className="text-sm text-muted-foreground">
            基于预占冻结与实耗结算机制，统一监控平台日月预算、后台入库子额度、四家提供商模型价目有效期与未知费用核对队列。
          </p>
          {summary?.paused && summary.pauseReason ? (
            <p className="text-xs font-medium text-foreground">
              当前暂停原因：{summary.pauseReason}
            </p>
          ) : null}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Dialog open={budgetOpen} onOpenChange={setBudgetOpen}>
            <DialogTrigger asChild>
              <Button variant="secondary" onClick={handleOpenBudgetDialog}>
                <SlidersHorizontalIcon data-icon="inline-start" />
                调整平台与账号预算
              </Button>
            </DialogTrigger>
            <DialogContent className="sm:max-w-xl">
              <DialogHeader>
                <DialogTitle>调整平台与账号 AI 预算上限</DialogTitle>
                <DialogDescription>
                  单位统一为「分」（100 分 = ¥1.00）。当已确认实耗 + 在途预占 +
                  待核对未知费用达到上限时，将自动阻断新请求。
                </DialogDescription>
              </DialogHeader>
              <form
                onSubmit={(e) => void handleSaveBudget(e)}
                className="space-y-5"
              >
                <FieldGroup>
                  <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                    <Field>
                      <FieldLabel htmlFor="b-platform-daily">
                        平台日预算上限（分）
                      </FieldLabel>
                      <Input
                        id="b-platform-daily"
                        type="number"
                        min={0}
                        value={platformDailyCents}
                        onChange={(e) => setPlatformDailyCents(e.target.value)}
                        disabled={savingBudget}
                        required
                      />
                      <FieldDescription>
                        折合 {formatCentsToYuan(Number(platformDailyCents))}
                      </FieldDescription>
                    </Field>
                    <Field>
                      <FieldLabel htmlFor="b-platform-monthly">
                        平台月预算上限（分）
                      </FieldLabel>
                      <Input
                        id="b-platform-monthly"
                        type="number"
                        min={0}
                        value={platformMonthlyCents}
                        onChange={(e) =>
                          setPlatformMonthlyCents(e.target.value)
                        }
                        disabled={savingBudget}
                        required
                      />
                      <FieldDescription>
                        折合 {formatCentsToYuan(Number(platformMonthlyCents))}
                      </FieldDescription>
                    </Field>
                  </div>

                  <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                    <Field>
                      <FieldLabel htmlFor="b-bg-daily">
                        后台知识入库日预算子额度（分）
                      </FieldLabel>
                      <Input
                        id="b-bg-daily"
                        type="number"
                        min={0}
                        value={backgroundDailyCents}
                        onChange={(e) =>
                          setBackgroundDailyCents(e.target.value)
                        }
                        disabled={savingBudget}
                        required
                      />
                      <FieldDescription>
                        折合 {formatCentsToYuan(Number(backgroundDailyCents))}
                        （超限仅暂停索引作业，不挤占前台问答）
                      </FieldDescription>
                    </Field>
                    <Field>
                      <FieldLabel htmlFor="b-alert-pct">
                        预算预警百分比阈值（%）
                      </FieldLabel>
                      <Input
                        id="b-alert-pct"
                        type="number"
                        min={1}
                        max={100}
                        value={alertThresholdPct}
                        onChange={(e) => setAlertThresholdPct(e.target.value)}
                        disabled={savingBudget}
                        required
                      />
                      <FieldDescription>
                        默认 80% 触发高水位预警徽标。
                      </FieldDescription>
                    </Field>
                  </div>

                  <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                    <Field>
                      <FieldLabel htmlFor="b-user-daily-cents">
                        单账号日预算上限（分）
                      </FieldLabel>
                      <Input
                        id="b-user-daily-cents"
                        type="number"
                        min={0}
                        value={userDailyCents}
                        onChange={(e) => setUserDailyCents(e.target.value)}
                        disabled={savingBudget}
                        required
                      />
                      <FieldDescription>
                        折合 {formatCentsToYuan(Number(userDailyCents))}
                      </FieldDescription>
                    </Field>
                    <Field>
                      <FieldLabel htmlFor="b-user-daily-q">
                        单账号日成功问答次数上限
                      </FieldLabel>
                      <Input
                        id="b-user-daily-q"
                        type="number"
                        min={1}
                        value={userDailyQuestions}
                        onChange={(e) => setUserDailyQuestions(e.target.value)}
                        disabled={savingBudget}
                        required
                      />
                      <FieldDescription>
                        失败或取消的请求不扣除成功次数。
                      </FieldDescription>
                    </Field>
                  </div>
                </FieldGroup>
                <DialogFooter>
                  <Button
                    type="button"
                    variant="secondary"
                    onClick={() => setBudgetOpen(false)}
                    disabled={savingBudget}
                  >
                    取消
                  </Button>
                  <Button type="submit" disabled={savingBudget}>
                    {savingBudget ? '保存中...' : '保存预算限额'}
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>

          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button
                variant={summary?.paused ? 'default' : 'destructive'}
                disabled={togglingPause}
              >
                {summary?.paused ? (
                  <>
                    <PlayCircleIcon data-icon="inline-start" />
                    恢复新增 AI 请求
                  </>
                ) : (
                  <>
                    <PauseCircleIcon data-icon="inline-start" />
                    暂停新增 AI 请求
                  </>
                )}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>
                  {summary?.paused
                    ? '确认恢复平台新增 AI 请求？'
                    : '确认暂停全平台新增 AI 请求？'}
                </AlertDialogTitle>
                <AlertDialogDescription>
                  {summary?.paused
                    ? '恢复后，符合四层预算与频次准入条件的问答及知识入库请求将立即恢复处理。'
                    : '暂停后，全平台所有新增问答提问与知识入库索引作业将立即被拒绝（返回 503 服务暂停），直至管理员手动恢复。'}
                </AlertDialogDescription>
              </AlertDialogHeader>
              {!summary?.paused ? (
                <Field className="py-2">
                  <FieldLabel htmlFor="pause-reason-input">
                    暂停原因说明
                  </FieldLabel>
                  <Input
                    id="pause-reason-input"
                    value={pauseReason}
                    onChange={(e) => setPauseReason(e.target.value)}
                    placeholder="填写暂停原因..."
                  />
                </Field>
              ) : null}
              <AlertDialogFooter>
                <AlertDialogCancel>取消</AlertDialogCancel>
                <AlertDialogAction
                  variant={summary?.paused ? 'default' : 'destructive'}
                  onClick={() => void handleTogglePause()}
                >
                  {summary?.paused ? '确认恢复准入' : '确认立即暂停'}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>

      {errorMsg ? (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>治理操作提示</AlertTitle>
          <AlertDescription>{errorMsg}</AlertDescription>
        </Alert>
      ) : null}

      {statusMsg ? (
        <Alert>
          <CheckCircle2Icon />
          <AlertTitle>配置已生效</AlertTitle>
          <AlertDescription>{statusMsg}</AlertDescription>
        </Alert>
      ) : null}

      {/* 4 Core Governance Metric Cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {/* Card 1: Platform Daily Budget */}
        <Card>
          <CardHeader className="pb-2">
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-medium text-muted-foreground">
                平台日预算（{summary?.date || '今日'}）
              </span>
              {summary?.dailyAlertTriggered || dailyPct >= 80 ? (
                <Badge variant="destructive">80% 预警</Badge>
              ) : (
                <Badge variant="outline">{dailyPct}%</Badge>
              )}
            </div>
            <CardTitle className="text-xl font-semibold">
              {formatCentsToYuan(dailyUsedTotal)}
              <span className="ml-1.5 text-xs font-normal text-muted-foreground">
                / 上限 {formatCentsToYuan(dailyLimit)}
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2.5">
            <Progress value={dailyPct} />
            <div className="space-y-1 text-xs text-muted-foreground">
              <div className="flex justify-between">
                <span>已确认实耗：</span>
                <span className="font-mono text-foreground">
                  {formatCentsToYuan(dailyConfirmed)}
                </span>
              </div>
              <div className="flex justify-between">
                <span>在途预占冻结：</span>
                <span className="font-mono text-foreground">
                  {formatCentsToYuan(dailyReserved)}
                </span>
              </div>
              <div className="flex justify-between">
                <span>待核对未知费用：</span>
                <span className="font-mono text-foreground">
                  {formatCentsToYuan(dailyUnknown)}
                </span>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Card 2: Platform Monthly Budget */}
        <Card>
          <CardHeader className="pb-2">
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-medium text-muted-foreground">
                平台月预算（{summary?.month || '本月'}）
              </span>
              {summary?.monthlyAlertTriggered || monthlyPct >= 80 ? (
                <Badge variant="destructive">80% 预警</Badge>
              ) : (
                <Badge variant="outline">{monthlyPct}%</Badge>
              )}
            </div>
            <CardTitle className="text-xl font-semibold">
              {formatCentsToYuan(monthlyUsedTotal)}
              <span className="ml-1.5 text-xs font-normal text-muted-foreground">
                / 上限 {formatCentsToYuan(monthlyLimit)}
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2.5">
            <Progress value={monthlyPct} />
            <div className="space-y-1 text-xs text-muted-foreground">
              <div className="flex justify-between">
                <span>月度已确认实耗：</span>
                <span className="font-mono text-foreground">
                  {formatCentsToYuan(monthlyConfirmed)}
                </span>
              </div>
              <div className="flex justify-between">
                <span>月度在途预占：</span>
                <span className="font-mono text-foreground">
                  {formatCentsToYuan(monthlyReserved)}
                </span>
              </div>
              <div className="flex justify-between">
                <span>待核对记录数：</span>
                <span className="font-mono text-foreground">
                  {summary?.pendingReconciliations ?? 0} 条
                </span>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Card 3: Background Indexing Sub-budget */}
        <Card>
          <CardHeader className="pb-2">
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-medium text-muted-foreground">
                后台入库子额度（今日）
              </span>
              <Badge variant="outline">{bgPct}%</Badge>
            </div>
            <CardTitle className="text-xl font-semibold">
              {formatCentsToYuan(bgUsedTotal)}
              <span className="ml-1.5 text-xs font-normal text-muted-foreground">
                / 子额度 {formatCentsToYuan(bgLimit)}
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2.5">
            <Progress value={bgPct} />
            <div className="space-y-1 text-xs text-muted-foreground">
              <div className="flex justify-between">
                <span>入库已确认实耗：</span>
                <span className="font-mono text-foreground">
                  {formatCentsToYuan(bgConfirmed)}
                </span>
              </div>
              <div className="flex justify-between">
                <span>入库在途预占：</span>
                <span className="font-mono text-foreground">
                  {formatCentsToYuan(bgReserved)}
                </span>
              </div>
              <p className="pt-0.5 text-[11px] text-muted-foreground">
                达到子额度上限时仅暂停后台切片作业（paused_budget）
              </p>
            </div>
          </CardContent>
        </Card>

        {/* Card 4: Concurrency & Personal Today Stats */}
        <Card>
          <CardHeader className="pb-2">
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-medium text-muted-foreground">
                当前并发与本人今日统计
              </span>
              <Badge variant="secondary">
                平台并发 {summary?.platformActiveRequests ?? 0}
              </Badge>
            </div>
            <CardTitle className="text-xl font-semibold">
              {summary?.questionsCount ?? 0}
              <span className="ml-1.5 text-xs font-normal text-muted-foreground">
                / {summary?.maxDailyQuestions ?? 0} 次本人今日问答
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-1.5 text-xs text-muted-foreground">
            <div className="flex justify-between">
              <span>本人今日在途请求：</span>
              <span className="font-mono text-foreground">
                {summary?.activeRequests ?? 0} 个（待结算{' '}
                {summary?.pendingQuestionsCount ?? 0}）
              </span>
            </div>
            <div className="flex justify-between">
              <span>本人今日费用占用：</span>
              <span className="font-mono text-foreground">
                {formatCentsToYuan(personalUsedCents)} /{' '}
                {formatCentsToYuan(personalLimitCents)}
              </span>
            </div>
            <div className="flex justify-between">
              <span>全平台当前活跃并发：</span>
              <span className="font-mono text-foreground">
                {summary?.platformActiveRequests ?? 0} 路运行中
              </span>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Tabs: Four-Provider Model Catalog & Unknown Cost Reconciliation Queue */}
      <Tabs defaultValue="models" className="w-full space-y-4">
        <TabsList>
          <TabsTrigger value="models">
            <CpuIcon data-icon="inline-start" />
            四家模型与价目目录 ({models.length})
          </TabsTrigger>
          <TabsTrigger value="reconciliations">
            <ReceiptTextIcon data-icon="inline-start" />
            未知费用核对队列 ({reconciliations.length})
          </TabsTrigger>
        </TabsList>

        <TabsContent value="models">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">
                受控模型目录（DeepSeek / OpenRouter / 火山引擎 / OpenAI）
              </CardTitle>
              <p className="text-xs text-muted-foreground">
                仅处于「允许调用」且价目版本在有效期内（未过期）的模型可被 Agent
                调用。任何模型停用或价目过期将自动在准入阶段拦截。
              </p>
            </CardHeader>
            <CardContent>
              {models.length === 0 ? (
                <Empty className="py-10">
                  <EmptyHeader>
                    <EmptyMedia variant="icon">
                      <CpuIcon />
                    </EmptyMedia>
                    <EmptyTitle>暂无模型目录记录</EmptyTitle>
                    <EmptyDescription>
                      服务端尚未返回受控模型目录。
                    </EmptyDescription>
                  </EmptyHeader>
                </Empty>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>提供商</TableHead>
                      <TableHead>模型标识与展示名称</TableHead>
                      <TableHead>启用状态</TableHead>
                      <TableHead>输入单价</TableHead>
                      <TableHead>输出单价</TableHead>
                      <TableHead>价目版本与有效期</TableHead>
                      <TableHead className="text-right">操作</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {models.map((m) => {
                      const expired = isModelExpired(m.expiresAt);
                      const isActive = m.enabled !== false && !expired;

                      return (
                        <TableRow key={`${m.providerId}:${m.modelId}`}>
                          <TableCell>
                            <div className="font-medium text-foreground">
                              {getProviderLabel(m.providerId)}
                            </div>
                            <div className="font-mono text-xs text-muted-foreground">
                              {m.providerId}
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="font-medium text-foreground">
                              {m.displayName || m.modelId}
                            </div>
                            <div className="font-mono text-xs text-muted-foreground">
                              {m.modelId}
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="flex flex-wrap items-center gap-1.5">
                              <Badge
                                variant={
                                  isActive
                                    ? 'default'
                                    : expired
                                      ? 'destructive'
                                      : 'secondary'
                                }
                              >
                                {m.enabled === false
                                  ? '已停用'
                                  : expired
                                    ? '价目已过期'
                                    : '允许调用'}
                              </Badge>
                            </div>
                          </TableCell>
                          <TableCell className="font-mono text-xs">
                            {formatPricePerK(m.promptPricePerK, m.currency)}
                          </TableCell>
                          <TableCell className="font-mono text-xs">
                            {formatPricePerK(m.completionPricePerK, m.currency)}
                          </TableCell>
                          <TableCell>
                            <div className="flex items-center gap-1.5 text-xs">
                              <Badge variant="outline">
                                {m.priceVersion || 'v1'}
                              </Badge>
                              <span className="text-muted-foreground">
                                {m.expiresAt
                                  ? `有效期至 ${formatDateTime(m.expiresAt)}`
                                  : '长期有效'}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell className="text-right">
                            <Button
                              variant="secondary"
                              size="sm"
                              onClick={() => handleOpenEditModel(m)}
                            >
                              <PencilIcon data-icon="inline-start" />
                              编辑启停与价目
                            </Button>
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="reconciliations">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">
                未知费用（Unknown Cost）核对队列
              </CardTitle>
              <p className="text-xs text-muted-foreground">
                当上游供应商响应超时或未返回精确 Token
                用量时，该笔请求的预占金额不会静默记为
                0，而是转入此队列继续占用日/月预算，直至自动对账或管理员人工核对结清。
              </p>
            </CardHeader>
            <CardContent>
              {reconciliations.length === 0 ? (
                <Empty className="py-10">
                  <EmptyHeader>
                    <EmptyMedia variant="icon">
                      <ReceiptTextIcon />
                    </EmptyMedia>
                    <EmptyTitle>当前无未知费用待核对记录</EmptyTitle>
                    <EmptyDescription>
                      所有 AI 问答与知识入库预占均已按精确回执完成结算。
                    </EmptyDescription>
                  </EmptyHeader>
                </Empty>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>核对单 ID / 预占单</TableHead>
                      <TableHead>触发用户</TableHead>
                      <TableHead>请求用途</TableHead>
                      <TableHead>冻结预占 / 核定实耗</TableHead>
                      <TableHead>核对状态</TableHead>
                      <TableHead>原因与创建时间</TableHead>
                      <TableHead className="text-right">操作</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {reconciliations.map((item) => (
                      <TableRow key={item.id}>
                        <TableCell>
                          <div className="font-mono text-xs font-medium text-foreground">
                            #{item.id}
                          </div>
                          {item.reservationId ? (
                            <div className="font-mono text-[11px] text-muted-foreground">
                              预占: {item.reservationId}
                            </div>
                          ) : null}
                        </TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">
                          {item.userId || 'system'}
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline">
                            {getPurposeLabel(item.purpose)}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <div className="font-mono text-xs text-foreground">
                            冻结: {formatCentsToYuan(item.unknownCents)}
                          </div>
                          {item.state === 'resolved' ? (
                            <div className="font-mono text-xs text-muted-foreground">
                              核定: {formatCentsToYuan(item.resolvedCents)}
                            </div>
                          ) : null}
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={
                              item.state === 'resolved'
                                ? 'secondary'
                                : 'default'
                            }
                          >
                            {getReconciliationStateLabel(item.state)}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <div className="max-w-xs truncate text-xs text-foreground">
                            {item.reason || '上游未返回有效用量回执'}
                          </div>
                          <div className="text-[11px] text-muted-foreground">
                            {formatDateTime(item.createdAt)}
                          </div>
                        </TableCell>
                        <TableCell className="text-right">
                          {item.state === 'resolved' ? (
                            <span className="text-xs text-muted-foreground">
                              已结清（{item.resolutionNote || '已核对'}）
                            </span>
                          ) : (
                            <Button
                              variant="secondary"
                              size="sm"
                              onClick={() => handleOpenResolve(item)}
                            >
                              人工结算核对
                            </Button>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Edit Model Status & Price Dialog */}
      <Dialog
        open={Boolean(editingModel)}
        onOpenChange={(open) => {
          if (!open) {
            setEditingModel(null);
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              编辑模型启停与价目（{editingModel?.providerId}:
              {editingModel?.modelId}）
            </DialogTitle>
            <DialogDescription>
              更新模型调用开关、千 Token
              输入/输出单价（分）、价目版本号及到期时间。
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={(e) => void handleSaveModel(e)} className="space-y-5">
            <FieldGroup>
              <Field>
                <label className="flex cursor-pointer items-center gap-2.5 rounded-lg bg-canvas-soft p-3 text-sm font-medium">
                  <Checkbox
                    checked={modelEnabled}
                    onCheckedChange={(val) => setModelEnabled(val === true)}
                    disabled={savingModel}
                  />
                  <span>允许在平台与 Agent 实例中调用该模型</span>
                </label>
              </Field>

              <Field>
                <FieldLabel htmlFor="m-display-name">模型展示名称</FieldLabel>
                <Input
                  id="m-display-name"
                  value={modelDisplayName}
                  onChange={(e) => setModelDisplayName(e.target.value)}
                  disabled={savingModel}
                  required
                />
              </Field>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <Field>
                  <FieldLabel htmlFor="m-prompt-price">
                    输入单价（分 / 千 Token）
                  </FieldLabel>
                  <Input
                    id="m-prompt-price"
                    type="number"
                    min={0}
                    step="0.01"
                    value={modelPromptPrice}
                    onChange={(e) => setModelPromptPrice(e.target.value)}
                    disabled={savingModel}
                    required
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="m-completion-price">
                    输出单价（分 / 千 Token）
                  </FieldLabel>
                  <Input
                    id="m-completion-price"
                    type="number"
                    min={0}
                    step="0.01"
                    value={modelCompletionPrice}
                    onChange={(e) => setModelCompletionPrice(e.target.value)}
                    disabled={savingModel}
                    required
                  />
                </Field>
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <Field>
                  <FieldLabel htmlFor="m-price-ver">价目版本号</FieldLabel>
                  <Input
                    id="m-price-ver"
                    value={modelPriceVersion}
                    onChange={(e) => setModelPriceVersion(e.target.value)}
                    disabled={savingModel}
                    required
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="m-expires-at">
                    价目有效期截止（ISO 时间）
                  </FieldLabel>
                  <Input
                    id="m-expires-at"
                    placeholder="例如：2027-12-31T23:59:59Z"
                    value={modelExpiresAt}
                    onChange={(e) => setModelExpiresAt(e.target.value)}
                    disabled={savingModel}
                  />
                </Field>
              </div>
            </FieldGroup>
            <DialogFooter>
              <Button
                type="button"
                variant="secondary"
                onClick={() => setEditingModel(null)}
                disabled={savingModel}
              >
                取消
              </Button>
              <Button type="submit" disabled={savingModel}>
                {savingModel ? '保存中...' : '保存模型配置'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Resolve Unknown Cost Reconciliation Dialog */}
      <Dialog
        open={Boolean(resolvingItem)}
        onOpenChange={(open) => {
          if (!open) {
            setResolvingItem(null);
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>人工结算核对未知费用 #{resolvingItem?.id}</DialogTitle>
            <DialogDescription>
              该记录原冻结预占金额为{' '}
              {formatCentsToYuan(resolvingItem?.unknownCents)}（
              {resolvingItem?.unknownCents ?? 0}{' '}
              分）。核定实际消耗金额后，差额将释放回对应周期的可用预算池。
            </DialogDescription>
          </DialogHeader>
          <form
            onSubmit={(e) => void handleResolveReconciliation(e)}
            className="space-y-5"
          >
            <FieldGroup>
              <Field>
                <FieldLabel htmlFor="rec-resolved-cents">
                  核定实际消耗金额（分）
                </FieldLabel>
                <Input
                  id="rec-resolved-cents"
                  type="number"
                  min={0}
                  value={resolvedCentsInput}
                  onChange={(e) => setResolvedCentsInput(e.target.value)}
                  disabled={resolving}
                  required
                />
                <FieldDescription>
                  折合 {formatCentsToYuan(Number(resolvedCentsInput))}
                  （输入 0 表示上游请求未实际计费，全额释放预占额度）
                </FieldDescription>
              </Field>
              <Field>
                <FieldLabel htmlFor="rec-note">核对备注说明</FieldLabel>
                <Textarea
                  id="rec-note"
                  placeholder="记录上游账单核对结论或释放原因..."
                  value={resolutionNote}
                  onChange={(e) => setResolutionNote(e.target.value)}
                  disabled={resolving}
                  rows={3}
                />
              </Field>
            </FieldGroup>
            <DialogFooter>
              <Button
                type="button"
                variant="secondary"
                onClick={() => setResolvingItem(null)}
                disabled={resolving}
              >
                取消
              </Button>
              <Button type="submit" disabled={resolving}>
                {resolving ? '提交核对中...' : '确认结清该笔费用'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}
