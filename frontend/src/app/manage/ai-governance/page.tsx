'use client';

import React, {useState, useEffect} from 'react';
import {
  getApiAssistantGovernanceSummary,
  getApiAssistantModels,
} from '@/services/generated/assistant/governance';
import {Card, CardHeader, CardTitle, CardContent} from '@/components/ui/card';
import {Badge} from '@/components/ui/badge';

export default function AiGovernancePage() {
  const [summary, setSummary] = useState<AssistantApi.UsageSummary | null>(
    null,
  );
  const [models, setModels] = useState<AssistantApi.ModelInfo[]>([]);

  useEffect(() => {
    let isMounted = true;
    void (async () => {
      try {
        const [sumRes, modelRes] = await Promise.all([
          getApiAssistantGovernanceSummary(),
          getApiAssistantModels(),
        ]);
        if (!isMounted) return;
        if (sumRes) setSummary(sumRes);
        if (Array.isArray(modelRes)) setModels(modelRes);
      } catch {
        // handle silently
      }
    })();
    return () => {
      isMounted = false;
    };
  }, []);

  return (
    <div className="container mx-auto max-w-5xl py-8 px-4 space-y-8">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">
          AI 费用与用量治理中心
        </h1>
        <p className="text-sm text-muted-foreground mt-1">
          统一监控 AI
          问答准入、单账号限额、平台日预算、并发控制及四家模型供应商接入情况。
        </p>
      </div>

      {/* Metrics Row */}
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <span className="text-xs text-muted-foreground">今日成功问答</span>
            <CardTitle className="text-2xl font-bold">
              {summary?.questionsCount ?? 0}
              <span className="text-xs font-normal text-muted-foreground ml-1">
                / {summary?.maxDailyQuestions ?? 20} 次
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-[11px] text-muted-foreground">
              单账号单日硬上限 20 次成功问答
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <span className="text-xs text-muted-foreground">
              个人今日预计花费
            </span>
            <CardTitle className="text-2xl font-bold">
              ¥{((summary?.totalCostCents ?? 0) / 100).toFixed(2)}
              <span className="text-xs font-normal text-muted-foreground ml-1">
                / ¥{((summary?.maxDailyCostCents ?? 200) / 100).toFixed(2)}
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-[11px] text-muted-foreground">
              单账号单日预算上限 ¥2.00
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <span className="text-xs text-muted-foreground">
              平台今日总花费
            </span>
            <CardTitle className="text-2xl font-bold">
              ¥{((summary?.platformCostCents ?? 0) / 100).toFixed(2)}
              <span className="text-xs font-normal text-muted-foreground ml-1">
                / ¥100.00
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-[11px] text-muted-foreground">
              平台单日综合预算上限 ¥100.00
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <span className="text-xs text-muted-foreground">
              当前活跃并发数
            </span>
            <CardTitle className="text-2xl font-bold">
              {summary?.activeRequests ?? 0}
              <span className="text-xs font-normal text-muted-foreground ml-1">
                / 1
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-[11px] text-muted-foreground">
              账号限制 1 并发，平台限制 10 并发
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Model Catalog Table */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">
            受控模型目录与费率标准 (四家提供商)
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs border-collapse">
              <thead>
                <tr className="border-b border-border text-muted-foreground">
                  <th className="py-2.5 px-3">供应商</th>
                  <th className="py-2.5 px-3">模型型号</th>
                  <th className="py-2.5 px-3">名称</th>
                  <th className="py-2.5 px-3">输入定价 (分/千Token)</th>
                  <th className="py-2.5 px-3">输出定价 (分/千Token)</th>
                  <th className="py-2.5 px-3">状态</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {models.map((m) => (
                  <tr
                    key={`${m.providerId}-${m.modelId}`}
                    className="hover:bg-muted/40"
                  >
                    <td className="py-2.5 px-3 font-medium uppercase">
                      {m.providerId}
                    </td>
                    <td className="py-2.5 px-3 font-mono text-[11px]">
                      {m.modelId}
                    </td>
                    <td className="py-2.5 px-3">{m.displayName}</td>
                    <td className="py-2.5 px-3">{m.promptPricePerK} 分/k</td>
                    <td className="py-2.5 px-3">
                      {m.completionPricePerK} 分/k
                    </td>
                    <td className="py-2.5 px-3">
                      <Badge
                        variant={m.enabled ? 'default' : 'secondary'}
                        className="text-[10px]"
                      >
                        {m.enabled ? '允许调用' : '已停用'}
                      </Badge>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
