import {Layout} from '@/components/layout/layout';
import Link from 'next/link';
import {ShieldAlertIcon} from 'lucide-react';
import {Button} from '@/components/ui/button';
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';
import type {SessionUser} from '@/lib/auth/roles';
import {buildLoginHref} from '@/lib/format';

export interface ManageAccessGuardProps {
  user: SessionUser | null;
  currentPath: string;
  requiredRoleLabel: string;
  title?: string;
  description?: string;
}

export function ManageAccessGuard({
  user,
  currentPath,
  requiredRoleLabel,
  title,
  description,
}: ManageAccessGuardProps) {
  const isLoggedIn = Boolean(user);

  return (
    <Layout flow="stack" className="mx-auto max-w-2xl py-12">
      <Empty className="bg-canvas-soft py-12">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <ShieldAlertIcon />
          </EmptyMedia>
          <EmptyTitle>
            {title ?? (isLoggedIn ? '无权访问该管理页面' : '请先登录后访问')}
          </EmptyTitle>
          <EmptyDescription>
            {description ??
              (isLoggedIn
                ? `当前账号不具备「${requiredRoleLabel}」权限，无法查看或操作此模块。`
                : `该页面仅对具备「${requiredRoleLabel}」权限的账号开放，请先登录。`)}
          </EmptyDescription>
        </EmptyHeader>
        <EmptyContent>
          <Layout
            flow="row"
            className="flex-wrap items-center justify-center gap-2"
          >
            {isLoggedIn ? (
              <Button variant="secondary" asChild>
                <Link href="/">返回首页</Link>
              </Button>
            ) : (
              <>
                <Button asChild>
                  <Link href={buildLoginHref(currentPath)}>前往登录</Link>
                </Button>
                <Button variant="secondary" asChild>
                  <Link href="/">返回首页</Link>
                </Button>
              </>
            )}
          </Layout>
        </EmptyContent>
      </Empty>
    </Layout>
  );
}
