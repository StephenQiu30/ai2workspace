import { DocumentPage } from '@/components/document';
import { getSource } from '@/lib/source';
export default async function Page({
  params,
  searchParams,
}: PageProps<'/read/[[...slug]]'>) {
  const { slug } = await params;
  const query = await searchParams;
  return (
    <DocumentPage
      space="self"
      slug={slug}
      revision={typeof query.revision === 'string' ? query.revision : undefined}
      legacy
    />
  );
}

export async function generateMetadata({
  params,
  searchParams,
}: PageProps<'/read/[[...slug]]'>) {
  const query = await searchParams;
  const context = await getSource(
    'self',
    typeof query.revision === 'string' ? query.revision : undefined,
    true,
  ).catch(() => undefined);
  return { title: context?.source.getPage((await params).slug)?.data.title };
}
