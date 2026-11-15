import { llms, loader } from 'fumadocs-core/source';
import { defineDocs } from 'fumadocs-mdx/macro';
import { metaSchema, pageSchema } from 'fumadocs-core/source/schema';
import documentIds from '@/content/document-ids.json';
import { docsRoute } from './shared';

const docs = defineDocs({
  dir: 'content',
  docs: {
    files: ['**/*.md'],
    schema: ({ source, path }) =>
      pageSchema.extend({
        title: pageSchema.shape.title
          .default(
            source.match(/^#\s+(.+)$/m)?.[1] ??
              path.split('/').at(-1)!.replace(/\.md$/, ''),
          )
          .transform((title) => {
            const number = path.match(/(?:^|\/)(\d{2})-/)?.[1];
            return number ? `${number} ${title}` : title;
          }),
      }),
    postprocess: { includeProcessedMarkdown: true },
  },
  meta: { files: ['**/meta.json'], schema: metaSchema },
});

export const source = loader({
  baseUrl: docsRoute,
  source: docs.toFumadocsSource(),
  slugs(file) {
    if (file.path === '00-文档索引.md') {
      return [];
    }
    const entry = documentIds.find((item) => item.path === file.path);
    if (!entry) {
      throw new Error(`文档缺少稳定 ID：${file.path}`);
    }
    return [entry.id];
  },
});

export const docsLlms = llms(source, {
  renderPage: (page) => page.data.getText('raw'),
});
