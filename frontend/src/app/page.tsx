import {ServiceStatus} from '../features/service_status/service_status';
import {projectName} from '../lib/project';
import {getServerRequestOptions} from '../lib/request/server';
import {getServiceHealth} from '../services/generated/ai2workspace/health';

export const dynamic = 'force-dynamic';

export default async function HomePage() {
  let health: Ai2workspaceApi.Response | null = null;
  try {
    health = await getServiceHealth(getServerRequestOptions());
  } catch {
    // A failed dependency must not prevent the page from offering recovery.
  }
  return (
    <main className="mx-auto flex min-h-svh w-full max-w-3xl flex-col justify-center gap-8 px-6 py-12">
      <header className="flex flex-col gap-3">
        <p className="text-sm text-muted-foreground">项目服务</p>
        <h1 className="text-3xl font-semibold tracking-tight">{projectName}</h1>
        <p className="text-muted-foreground">
          连接项目服务，开始后续业务开发。
        </p>
      </header>
      <ServiceStatus initialHealth={health} />
    </main>
  );
}
