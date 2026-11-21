import Link from 'next/link';
import {ArrowUpRight, BookOpen} from 'lucide-react';
import {Badge} from '@/components/ui/badge';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {ServiceStatus} from '@/features/service_status/service_status';
import {
  SERVICE_CATALOG,
  checkAllServicesHealth,
  createUnavailableRecord,
  type ServiceHealthRecord,
} from '@/features/service_status/service_registry';
import {projectName} from '@/lib/project';
import {getServerRequestOptions} from '@/lib/request/server';

export const dynamic = 'force-dynamic';

const PRIMARY_ENTRIES = [
  {label: '讨论社区', href: '/community'},
  {label: '课程广场', href: '/courses'},
  {label: '我的订单', href: '/orders'},
  {label: '站内通知', href: '/notifications'},
  {label: '账号中心', href: '/account'},
] as const;

export default async function HomePage() {
  let records: ServiceHealthRecord[];
  try {
    const options = await getServerRequestOptions();
    records = await checkAllServicesHealth(options);
  } catch {
    records = SERVICE_CATALOG.map((service) =>
      createUnavailableRecord(service.id),
    );
  }

  return (
    <main className="mx-auto flex min-h-svh w-full max-w-5xl flex-col gap-8 px-4 py-10 sm:px-6 sm:py-12">
      <header className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-sm text-muted-foreground">项目服务总览</p>
          <Badge variant="secondary" className="font-mono text-xs">
            gateway :3121
          </Badge>
        </div>
        <div className="flex flex-col gap-2">
          <h1 className="text-3xl font-semibold tracking-tight">
            {projectName}
          </h1>
          <p className="text-sm text-muted-foreground sm:text-base">
            记录当前 5 个业务微服务与知识库模块的实时健康状态、Swagger
            接口契约及业务页面入口。
          </p>
        </div>
        <nav aria-label="快捷业务入口" className="flex flex-wrap gap-2 pt-1">
          {PRIMARY_ENTRIES.map((entry) => (
            <Link
              key={entry.href}
              href={entry.href}
              className="inline-flex min-h-11 items-center gap-1 rounded-md bg-primary px-3.5 py-2 text-sm font-medium text-primary-foreground transition-opacity hover:opacity-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring sm:min-h-9"
            >
              {entry.label}
              <ArrowUpRight className="size-4" aria-hidden="true" />
            </Link>
          ))}
          <a
            href="http://127.0.0.1:3122/docs/"
            target="_blank"
            rel="noreferrer"
            className="inline-flex min-h-11 items-center gap-1.5 rounded-md bg-secondary px-3.5 py-2 text-sm font-medium text-secondary-foreground transition-colors hover:bg-secondary/80 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring sm:min-h-9"
          >
            <BookOpen className="size-4" aria-hidden="true" />
            知识库（:3122）
          </a>
        </nav>
      </header>

      <ServiceStatus initialRecords={records} />

      <Card>
        <CardHeader>
          <CardTitle className="text-base">本地运行拓扑</CardTitle>
          <CardDescription>
            端口分配与 `PROJECT.md` 保持一致；前端页面与 API 统一经 Traefik
            网关（:3121）分发。
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
            <div className="flex items-center justify-between rounded-lg bg-muted/50 px-3.5 py-2.5">
              <div>
                <p className="font-mono font-medium">frontend / gateway</p>
                <p className="text-xs text-muted-foreground">
                  Next.js 业务前端与 Traefik 统一入口
                </p>
              </div>
              <Badge variant="outline" className="font-mono text-xs">
                :3121
              </Badge>
            </div>
            <div className="flex items-center justify-between rounded-lg bg-muted/50 px-3.5 py-2.5">
              <div>
                <p className="font-mono font-medium">workspace</p>
                <p className="text-xs text-muted-foreground">
                  Fumadocs 静态知识库站点
                </p>
              </div>
              <Badge variant="outline" className="font-mono text-xs">
                :3122
              </Badge>
            </div>
          </div>
        </CardContent>
      </Card>
    </main>
  );
}
