import type {Metadata} from 'next';
import {redirect} from 'next/navigation';
import {isAxiosError} from 'axios';
import {CircleAlertIcon} from 'lucide-react';
import {Alert, AlertDescription, AlertTitle} from '@/components/ui/alert';
import {DashboardEmpty, DashboardFrame} from '@/features/studio/dashboard';
import {RefreshWorkspaceButton} from '@/features/studio/dashboard_sheet';
import {buildLoginHref} from '@/lib/format';
import {getServerSession} from '@/lib/auth/session';
import {getServerRequestOptions} from '@/lib/request/server';
import {studioGetWorkspaceAccess} from '@/services/generated/api/studio';

export const metadata: Metadata = {
  title: '制作工作台',
  robots: {index: false, follow: false},
};

interface DashboardPageProps {
  searchParams: Promise<{workspace?: string | string[]}>;
}

export default async function DashboardPage({
  searchParams,
}: DashboardPageProps) {
  const {workspace} = await searchParams;
  const configuredWorkspace = workspace ?? process.env.STUDIO_WORKSPACE_ID;
  const workspaceId =
    typeof configuredWorkspace === 'string' &&
    /^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$/.test(configuredWorkspace)
      ? configuredWorkspace
      : undefined;
  const nextPath =
    typeof workspace === 'string' && workspaceId
      ? `/dashboard?workspace=${workspaceId}`
      : '/dashboard';
  const loginHref = buildLoginHref(nextPath);
  const options = await getServerRequestOptions();

  if (!options.headers?.Cookie) redirect(loginHref);

  if (!workspaceId) {
    if (!(await getServerSession())) redirect(loginHref);
    return (
      <DashboardFrame>
        <DashboardEmpty
          title="暂无可用的制作工作区"
          description="请从有效的工作区链接进入，或联系工作区负责人。"
        />
      </DashboardFrame>
    );
  }

  try {
    await studioGetWorkspaceAccess({workspaceId}, options);
  } catch (error: unknown) {
    const status = isAxiosError(error) ? error.response?.status : undefined;
    if (status === 401) redirect(loginHref);
    if (status === 403) {
      return (
        <DashboardFrame>
          <DashboardEmpty
            title="当前无法访问此工作区"
            description="请确认工作区邀请或联系负责人核对权限。"
          />
        </DashboardFrame>
      );
    }
    return (
      <DashboardFrame>
        <Alert variant="destructive">
          <CircleAlertIcon aria-hidden />
          <AlertTitle>暂时无法读取工作区</AlertTitle>
          <AlertDescription>
            <RefreshWorkspaceButton />
          </AlertDescription>
        </Alert>
      </DashboardFrame>
    );
  }

  return (
    <DashboardFrame>
      <DashboardEmpty
        title="暂无制作项目"
        description="工作区已可访问，制作项目功能准备中。"
      />
    </DashboardFrame>
  );
}
