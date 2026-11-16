import Link from 'next/link';
import type { ReactNode } from 'react';
import { buttonVariants } from '@/components/ui/button';
export function SpaceShell({
  children,
  title,
  description,
}: {
  children: ReactNode;
  title: string;
  description?: string;
}) {
  return (
    <main className="mx-auto flex w-full max-w-5xl flex-col gap-8 px-6 py-10">
      <nav className="flex flex-wrap items-center gap-3" aria-label="空间导航">
        <Link href="/read" className={buttonVariants({ variant: 'ghost' })}>
          项目文档
        </Link>
        <Link
          href="/spaces/self"
          prefetch={false}
          className={buttonVariants({ variant: 'ghost' })}
        >
          任务与交接
        </Link>
        <Link
          href="/spaces/self/settings"
          prefetch={false}
          className={buttonVariants({ variant: 'outline' })}
        >
          Workspace 配置
        </Link>
      </nav>
      <header className="flex flex-col gap-2">
        <h1 className="text-3xl font-semibold tracking-tight">{title}</h1>
        {description ? (
          <p className="text-muted-foreground">{description}</p>
        ) : null}
      </header>
      {children}
    </main>
  );
}
