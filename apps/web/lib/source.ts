import { llms } from 'fumadocs-core/source';
import { readWorkspace } from './workspaces';
import { buildKnowledgeSource } from './knowledge-source';
import { cache } from 'react';

export const getSource = cache(async function getSource(
  key = 'self',
  revision?: string,
  legacy = false,
) {
  const view = await readWorkspace(key, revision);
  const baseUrl = legacy ? '/read' : `/spaces/${key}/read`;
  const source = buildKnowledgeSource(
    view.scan,
    view.workspace.definition.sources,
    baseUrl,
    legacy,
  );
  const text = llms(source, {
    renderPage: (page) => page.data.document.body,
    renderUrl: (item) => {
      const url = new URL(item.url, 'http://localhost');
      url.pathname += '.md';
      return `${url.pathname}${url.search}${url.hash}`;
    },
  });
  return { ...view, source, text, baseUrl };
});
