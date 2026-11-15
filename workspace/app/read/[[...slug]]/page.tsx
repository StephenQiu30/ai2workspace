import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import {
  DocsBody,
  DocsPage,
  DocsTitle,
  MarkdownCopyButton,
} from 'fumadocs-ui/layouts/docs/page';
import { createRelativeLink } from 'fumadocs-ui/mdx';
import { getMDXComponents } from '@/components/mdx';
import { source } from '@/lib/source';

export default async function Page(props: PageProps<'/read/[[...slug]]'>) {
  const params = await props.params;
  const page = source.getPage(params.slug);
  if (!page) {
    notFound();
  }

  const MDX = page.data.body;
  return (
    <DocsPage toc={page.data.toc} full={page.data.full}>
      <div className="flex border-b pb-6">
        <MarkdownCopyButton markdownUrl={`${page.url}.md`} />
      </div>
      <DocsBody>
        <MDX
          components={getMDXComponents({
            a: createRelativeLink(source, page),
            h1: (props) => <DocsTitle {...props}>{page.data.title}</DocsTitle>,
          })}
        />
      </DocsBody>
    </DocsPage>
  );
}

export function generateStaticParams() {
  return source.generateParams();
}

export async function generateMetadata(
  props: PageProps<'/read/[[...slug]]'>,
): Promise<Metadata> {
  const params = await props.params;
  const page = source.getPage(params.slug);
  if (!page) {
    notFound();
  }
  return { title: page.data.title, description: page.data.description };
}
