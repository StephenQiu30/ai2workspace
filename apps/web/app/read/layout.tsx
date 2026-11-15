import { KnowledgeLayout } from '@/components/knowledge-layout';
export const dynamic = 'force-dynamic';
export default function Layout({ children }: LayoutProps<'/read'>) {
  return (
    <KnowledgeLayout space="self" legacy>
      {children}
    </KnowledgeLayout>
  );
}
