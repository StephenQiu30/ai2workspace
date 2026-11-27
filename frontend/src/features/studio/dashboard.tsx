import type {ReactNode} from 'react';
import Link from 'next/link';
import {ClapperboardIcon} from 'lucide-react';
import {Layout} from '@/components/layout/layout';
import {Heading, Text} from '@/components/layout/typography';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty';

export function DashboardFrame({
  children,
  preview = false,
}: {
  children: ReactNode;
  preview?: boolean;
}) {
  return (
    <Layout flow="stack" className="gap-6 pt-2 md:gap-10 md:pt-4">
      <Layout as="header" flow="stack" className="items-start gap-2 md:gap-3">
        <Layout flow="row" className="flex-wrap items-center gap-3">
          <Heading
            level={1}
            className="text-[1.9375rem]/tight font-bold md:text-[2.75rem]/tight"
          >
            制作工作台
          </Heading>
          {preview ? <Badge variant="secondary">界面预览</Badge> : null}
        </Layout>
        <Text
          as="p"
          className="text-sm leading-relaxed text-muted-foreground md:text-base"
        >
          {preview
            ? '示例项目及待处理事项，可查看制作和审片界面。'
            : '继续当前制作，处理需要你确认的事项。'}
        </Text>
      </Layout>
      {children}
    </Layout>
  );
}

export function DashboardEmpty({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle as="h2">制作项目</CardTitle>
      </CardHeader>
      <CardContent>
        <Empty>
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <ClapperboardIcon aria-hidden />
            </EmptyMedia>
            <EmptyTitle>{title}</EmptyTitle>
            <EmptyDescription>{description}</EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button variant="outline" size="lg" asChild>
              <Link href="/dashboard/preview">查看界面预览</Link>
            </Button>
          </EmptyContent>
        </Empty>
      </CardContent>
    </Card>
  );
}
