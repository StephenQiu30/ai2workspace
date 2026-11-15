import { MarkdownAsync } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { remarkHeading, rehypeCode } from 'fumadocs-core/mdx-plugins';
import type { TOCItemType } from 'fumadocs-core/toc';
import defaultComponents from 'fumadocs-ui/mdx';
import {
  DocsBody,
  DocsPage,
  MarkdownCopyButton,
  DocsTitle,
} from 'fumadocs-ui/layouts/docs/page';
import { Suspense, isValidElement, type ReactNode } from 'react';
import Link from 'next/link';
import { notFound, redirect } from 'next/navigation';
import { getSource } from '@/lib/source';
import {
  documentUrl,
  resolveAttachmentPath,
  resolveDocumentLink,
} from '@/lib/source-links';
import { Alert, AlertTitle, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Mermaid } from '@/components/mdx/mermaid';

function plainText(node: ReactNode): string {
  if (typeof node === 'string' || typeof node === 'number') {
    return String(node);
  }
  if (Array.isArray(node)) {
    return node.map(plainText).join('');
  }
  if (isValidElement<{ children?: ReactNode }>(node)) {
    return plainText(node.props.children);
  }
  return '';
}

export async function DocumentPage({
  space,
  slug,
  revision,
  legacy = false,
}: {
  space: string;
  slug?: string[];
  revision?: string;
  legacy?: boolean;
}) {
  const context = await getSource(space, revision, legacy).catch(() =>
    notFound(),
  );
  const page = context.source.getPage(slug);
  if (!page) {
    notFound();
  }
  const document = page.data.document;
  if (!revision) {
    redirect(
      documentUrl(document, context.baseUrl, context.scan.sourceRevision),
    );
  }
  const result: { toc: TOCItemType[] } = { toc: [] };
  const content = await MarkdownAsync({
    children: document.body,
    skipHtml: true,
    remarkPlugins: [
      remarkGfm,
      remarkHeading,
      () => (_tree, file) => {
        result.toc.push(...(file.data.toc ?? []));
      },
    ],
    rehypePlugins: [
      [
        rehypeCode,
        {
          themes: { light: 'github-light', dark: 'github-dark' },
          fallbackLanguage: 'text',
          addLanguageClass: true,
        },
      ],
    ],
    components: {
      h1: ({ children, id }) => <DocsTitle id={id}>{children}</DocsTitle>,
      h2: ({ children, id }) => (
        <defaultComponents.h2 id={id}>{children}</defaultComponents.h2>
      ),
      h3: ({ children, id }) => (
        <defaultComponents.h3 id={id}>{children}</defaultComponents.h3>
      ),
      h4: ({ children, id }) => (
        <defaultComponents.h4 id={id}>{children}</defaultComponents.h4>
      ),
      h5: ({ children, id }) => (
        <defaultComponents.h5 id={id}>{children}</defaultComponents.h5>
      ),
      h6: ({ children, id }) => (
        <defaultComponents.h6 id={id}>{children}</defaultComponents.h6>
      ),
      table: ({ children }) => (
        <defaultComponents.table>{children}</defaultComponents.table>
      ),
      a: ({ href, children }) => {
        const resolved =
          href &&
          resolveDocumentLink(
            href,
            document,
            context.scan.documents,
            context.baseUrl,
            revision,
            Object.fromEntries(
              context.workspace.definition.sources.flatMap((source) => {
                const root = context.workspace.bindings[source.binding];
                return root ? [[source.id, root]] : [];
              }),
            ),
          );
        return resolved ? (
          <defaultComponents.a href={resolved}>{children}</defaultComponents.a>
        ) : (
          <span>{children}</span>
        );
      },
      img: ({ src, alt }) => {
        if (typeof src !== 'string') {
          return null;
        }
        const attachment = resolveAttachmentPath(src, document.relativePath);
        return attachment ? (
          <defaultComponents.img
            src={`/api/spaces/${space}/assets/${document.sourceId}/${attachment.split('/').map(encodeURIComponent).join('/')}?revision=${revision}`}
            alt={alt ?? ''}
          />
        ) : (
          <span>{alt ?? '图片未发布'}</span>
        );
      },
      pre: ({ children, node, ...props }) => {
        void node;
        if (
          isValidElement<{ className?: string; children?: ReactNode }>(
            children,
          ) &&
          children.props.className?.split(/\s+/).includes('language-mermaid')
        ) {
          return (
            <Suspense fallback={<pre>{children}</pre>}>
              <Mermaid chart={plainText(children.props.children)} />
            </Suspense>
          );
        }
        return (
          <defaultComponents.pre {...props}>{children}</defaultComponents.pre>
        );
      },
    },
  });
  const textUrl = new URL(
    documentUrl(document, context.baseUrl, revision),
    'http://localhost',
  );
  return (
    <DocsPage toc={result.toc}>
      <div className="flex flex-wrap items-center gap-3 border-b pb-4 text-sm text-muted-foreground">
        <Badge variant="secondary">{document.metadata.status}</Badge>
        <span>
          来源：{document.sourceId} / {document.relativePath}
        </span>
        <span title={context.scan.sourceRevision}>
          修订 {context.scan.sourceRevision.slice(0, 12)}
        </span>
        <MarkdownCopyButton
          markdownUrl={`${textUrl.pathname}.md${textUrl.search}`}
        />
      </div>
      {context.stale ? (
        <Alert>
          <AlertTitle>正在阅读旧修订</AlertTitle>
          <AlertDescription>
            知识来源或当前内容已变化。此页、搜索和 Markdown 保持同一修订。
            <Link href={documentUrl(document, context.baseUrl)}>
              读取最新知识
            </Link>
          </AlertDescription>
        </Alert>
      ) : null}
      <DocsBody>{content}</DocsBody>
    </DocsPage>
  );
}
