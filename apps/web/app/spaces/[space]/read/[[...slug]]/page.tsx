import { DocumentPage } from '@/components/document';
import { getSource } from '@/lib/source';
import { redirect } from 'next/navigation';
export default async function Page({
  params,
  searchParams,
}: PageProps<'/spaces/[space]/read/[[...slug]]'>) {
  const { space, slug } = await params;
  const query = await searchParams;
  if (!slug?.length) {
    const { source } = await getSource(space);
    const first = source.getPages()[0];
    if (first) {
      redirect(first.url);
    }
  }
  return (
    <DocumentPage
      space={space}
      slug={slug}
      revision={typeof query.revision === 'string' ? query.revision : undefined}
    />
  );
}

export async function generateMetadata({
  params,
  searchParams,
}: PageProps<'/spaces/[space]/read/[[...slug]]'>) {
  const { space, slug } = await params;
  const query = await searchParams;
  const context = await getSource(
    space,
    typeof query.revision === 'string' ? query.revision : undefined,
  ).catch(() => undefined);
  return { title: context?.source.getPage(slug)?.data.title };
}
