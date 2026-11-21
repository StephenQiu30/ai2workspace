'use client';

import Link from 'next/link';
import {useState} from 'react';
import {
  AlertCircle,
  ArrowUpRight,
  CheckCircle2,
  ExternalLink,
  RefreshCw,
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
  SERVICE_CATALOG,
  checkAllServicesHealth,
  checkServiceHealth,
  createUnavailableRecord,
  formatCheckedTime,
  summarizeServiceHealth,
  type ServiceHealthRecord,
  type ServiceId,
} from './service_registry';

interface ServiceStatusProps {
  initialRecords: ServiceHealthRecord[];
}

export function ServiceStatus({initialRecords}: ServiceStatusProps) {
  const [records, setRecords] = useState<ServiceHealthRecord[]>(initialRecords);
  const [refreshingAll, setRefreshingAll] = useState(false);
  const [pendingIds, setPendingIds] = useState<Record<string, boolean>>({});

  const recordMap = new Map<ServiceId, ServiceHealthRecord>(
    records.map((record) => [record.id, record]),
  );

  const normalizedRecords = SERVICE_CATALOG.map(
    (meta) => recordMap.get(meta.id) ?? createUnavailableRecord(meta.id),
  );

  const summary = summarizeServiceHealth(normalizedRecords);
  const latestCheckedAt = normalizedRecords.reduce<string>(
    (latest, record) => (record.checkedAt > latest ? record.checkedAt : latest),
    '',
  );

  async function refreshAll() {
    setRefreshingAll(true);
    try {
      const next = await checkAllServicesHealth();
      setRecords(next);
    } finally {
      setRefreshingAll(false);
    }
  }

  async function refreshOne(id: ServiceId) {
    setPendingIds((prev) => ({...prev, [id]: true}));
    try {
      const updated = await checkServiceHealth(id);
      setRecords((prev) => {
        const exists = prev.some((item) => item.id === id);
        if (!exists) {
          return [...prev, updated];
        }
        return prev.map((item) => (item.id === id ? updated : item));
      });
    } finally {
      setPendingIds((prev) => ({...prev, [id]: false}));
    }
  }

  return (
    <section className="flex flex-col gap-6" aria-label="微服务运行状态">
      <Card>
        <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex flex-col gap-1.5">
            <CardTitle>微服务实时状态</CardTitle>
            <CardDescription>
              通过统一网关检查全部 {summary.total} 个业务微服务的真实健康记录
              {latestCheckedAt
                ? ` · 最近检查 ${formatCheckedTime(latestCheckedAt)}`
                : ''}
            </CardDescription>
          </div>
          <Button
            variant="outline"
            className="min-h-11 shrink-0"
            disabled={refreshingAll}
            onClick={refreshAll}
          >
            <RefreshCw
              className={`size-4 ${refreshingAll ? 'animate-spin' : ''}`}
              aria-hidden="true"
            />
            {refreshingAll ? '检查中…' : '重新检查全部'}
          </Button>
        </CardHeader>
        <CardContent aria-live="polite" aria-busy={refreshingAll}>
          {summary.allHealthy ? (
            <div className="flex flex-wrap items-center gap-3">
              <Badge variant="secondary" className="gap-1.5 px-3 py-1">
                <CheckCircle2 className="size-3.5" aria-hidden="true" />
                全部服务正常（{summary.healthy}/{summary.total}）
              </Badge>
              <span className="text-sm text-muted-foreground">
                统一入口经 Traefik（:3121）按服务前缀分发至 5 个独立 Go 微服务。
              </span>
            </div>
          ) : (
            <Alert variant="destructive">
              <AlertCircle className="size-4" aria-hidden="true" />
              <AlertTitle>
                部分服务暂不可用（正常 {summary.healthy}/{summary.total}）
              </AlertTitle>
              <AlertDescription>
                不可用服务：
                {normalizedRecords
                  .filter((record) => !record.available)
                  .map((record) => record.id)
                  .join('、')}
                。请核查对应容器或稍后重试。
              </AlertDescription>
            </Alert>
          )}
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {SERVICE_CATALOG.map((service) => {
          const record =
            recordMap.get(service.id) ?? createUnavailableRecord(service.id);
          const isPending = refreshingAll || Boolean(pendingIds[service.id]);

          return (
            <Card key={service.id} className="flex flex-col justify-between">
              <CardHeader className="gap-2">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <CardTitle className="font-mono text-base">
                      {service.id}
                    </CardTitle>
                    <Badge variant="outline" className="font-mono text-xs">
                      :{service.port}
                    </Badge>
                  </div>
                  <Badge
                    variant={record.available ? 'secondary' : 'destructive'}
                    className="gap-1"
                  >
                    {record.available ? (
                      <>
                        <CheckCircle2 className="size-3" aria-hidden="true" />
                        连接正常
                      </>
                    ) : (
                      <>
                        <AlertCircle className="size-3" aria-hidden="true" />
                        暂不可用
                      </>
                    )}
                  </Badge>
                </div>
                <p className="text-sm font-medium text-foreground">
                  {service.title}
                </p>
                <CardDescription>{service.description}</CardDescription>
              </CardHeader>

              <CardContent className="flex flex-col gap-3">
                <dl className="grid grid-cols-2 gap-x-4 gap-y-2 rounded-lg bg-muted/50 px-3 py-2.5 text-xs">
                  <div className="flex flex-col gap-0.5">
                    <dt className="text-muted-foreground">健康接口</dt>
                    <dd className="truncate font-mono text-foreground">
                      {service.healthPath}
                    </dd>
                  </div>
                  <div className="flex flex-col gap-0.5">
                    <dt className="text-muted-foreground">服务响应</dt>
                    <dd className="font-mono text-foreground">
                      {record.available
                        ? `${record.reportedService ?? service.id} · ${record.status}`
                        : record.status}
                    </dd>
                  </div>
                  <div className="flex flex-col gap-0.5">
                    <dt className="text-muted-foreground">响应耗时</dt>
                    <dd className="font-mono text-foreground">
                      {record.latencyMs !== null
                        ? `${record.latencyMs} ms`
                        : '--'}
                    </dd>
                  </div>
                  <div className="flex flex-col gap-0.5">
                    <dt className="text-muted-foreground">检查时间</dt>
                    <dd className="font-mono text-foreground">
                      {formatCheckedTime(record.checkedAt)}
                    </dd>
                  </div>
                </dl>

                <div className="flex flex-wrap gap-2">
                  {service.routes.map((route) => (
                    <Link
                      key={route.href}
                      href={route.href}
                      className="inline-flex min-h-9 items-center gap-1 rounded-md bg-secondary px-3 py-1.5 text-xs font-medium text-secondary-foreground transition-colors hover:bg-secondary/80 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                    >
                      {route.label}
                      <ArrowUpRight className="size-3.5" aria-hidden="true" />
                    </Link>
                  ))}
                </div>
              </CardContent>

              <CardFooter className="flex flex-wrap items-center justify-between gap-2 pt-0">
                <Button
                  variant="outline"
                  size="sm"
                  className="min-h-11 sm:min-h-9"
                  disabled={isPending}
                  onClick={() => refreshOne(service.id)}
                >
                  <RefreshCw
                    className={`size-3.5 ${isPending ? 'animate-spin' : ''}`}
                    aria-hidden="true"
                  />
                  {isPending ? '检查中…' : '重新检查'}
                </Button>
                <a
                  href={service.swaggerPath}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex min-h-11 items-center gap-1 px-2 text-xs font-medium text-foreground underline underline-offset-4 hover:text-muted-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring sm:min-h-9"
                >
                  Swagger 接口文档
                  <ExternalLink className="size-3.5" aria-hidden="true" />
                </a>
              </CardFooter>
            </Card>
          );
        })}
      </div>
    </section>
  );
}
