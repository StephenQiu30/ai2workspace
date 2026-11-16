import { DocsLayout } from 'fumadocs-ui/layouts/docs';
import { headers } from 'next/headers';
import { notFound } from 'next/navigation';
import type { ReactNode } from 'react';
import { getSource } from '@/lib/source';
import { baseOptions } from '@/lib/layout.shared';

export async function KnowledgeLayout({
  space,
  children,
  legacy = false,
}: {
  space: string;
  children: ReactNode;
  legacy?: boolean;
}) {
  const revision =
    (await headers()).get('x-ai2workspace-revision') ?? undefined;
  const { source } = await getSource(space, revision, legacy).catch(() =>
    notFound(),
  );
  return (
    <DocsLayout
      tree={source.getPageTree()}
      {...baseOptions()}
      links={[
        { text: '项目文档', url: '/read' },
        { text: '任务与交接', url: `/spaces/${space}` },
        { text: 'Workspace 配置', url: `/spaces/${space}/settings` },
      ]}
    >
      {children}
    </DocsLayout>
  );
}
