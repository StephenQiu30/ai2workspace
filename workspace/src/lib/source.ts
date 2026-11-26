import { llms, loader } from 'fumadocs-core/source';
import { docsRoute } from './shared';
import { defineDocs } from 'fumadocs-mdx/macro';
import { metaSchema, pageSchema } from 'fumadocs-core/source/schema';

const docs = defineDocs({
  dir: '.',
  docs: {
    files: ['01-项目总体设计.md', '02-项目工程说明.md', 'content/**/*.md', 'content/**/*.mdx'],
    schema: pageSchema,
    postprocess: {
      includeProcessedMarkdown: true,
    },
  },
  meta: {
    files: ['meta.json', 'content/**/meta.json'],
    schema: metaSchema,
  },
});

// See https://fumadocs.dev/docs/headless/source-api for more info
export const source = loader({
  baseUrl: docsRoute,
  source: docs.toFumadocsSource(),
  slugs(file, next) {
    if (file.path === '01-项目总体设计.md') return ['architecture'];
    if (file.path === '02-项目工程说明.md') return ['project'];
    if (file.path === 'content/00-文档导航.md') return [];
    return next().slice(1);
  },
});

export const docsLlms = llms(source, {
  renderPage: async (page) => `# ${page.data.title} (${page.url})

${await page.data.getText('processed')}`,
  // link to the files under `/llms.mdx`
  renderUrl: (item) => (/^\/.*\.(md|txt)$/.test(item.url) ? `/llms.mdx${item.url}` : item.url),
});
