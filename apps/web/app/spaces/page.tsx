import Link from 'next/link';
import { listWorkspaces } from '@/lib/workspaces';
import { SpaceShell } from '@/components/space-shell';
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from '@/components/ui/card';
import { buttonVariants } from '@/components/ui/button';
export const dynamic = 'force-dynamic';
export default async function Page() {
  const spaces = await listWorkspaces();
  return (
    <SpaceShell
      title="知识空间"
      description="选择本机已登记空间，阅读知识、查看治理与交接。"
    >
      <div className="grid gap-4 sm:grid-cols-2">
        {spaces.map((space) => (
          <Card key={space.key}>
            <CardHeader>
              <CardTitle>{space.name}</CardTitle>
              <CardDescription>{space.id ?? space.key}</CardDescription>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">
                {space.issue ?? '使用独立来源、知识修订与任务记录'}
              </p>
            </CardContent>
            <CardFooter className="flex flex-wrap gap-2">
              <Link
                className={buttonVariants({ variant: 'outline' })}
                href={`/spaces/${space.key}/read`}
              >
                阅读知识
              </Link>
              <Link
                className={buttonVariants({ variant: 'ghost' })}
                href={`/spaces/${space.key}`}
              >
                治理与交接
              </Link>
              <Link
                className={buttonVariants({ variant: 'ghost' })}
                href={`/spaces/${space.key}/settings`}
              >
                配置
              </Link>
            </CardFooter>
          </Card>
        ))}
      </div>
      {spaces.length === 0 ? (
        <p>尚未登记空间。创建声明后，在本机配置知识目录绑定。</p>
      ) : null}
    </SpaceShell>
  );
}
