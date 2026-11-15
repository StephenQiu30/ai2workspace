import { KnowledgeLayout } from '@/components/knowledge-layout';
export const dynamic = 'force-dynamic';
export default async function Layout({
  children,
  params,
}: LayoutProps<'/spaces/[space]/read'>) {
  return (
    <KnowledgeLayout space={(await params).space}>{children}</KnowledgeLayout>
  );
}
