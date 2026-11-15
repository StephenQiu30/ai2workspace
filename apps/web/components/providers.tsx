'use client';

import { RootProvider } from 'fumadocs-ui/provider/next';
import DefaultSearchDialog from 'fumadocs-ui/components/dialog/search-default';
import { usePathname, useSearchParams } from 'next/navigation';
import type { ReactNode } from 'react';

export function Providers({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const params = useSearchParams();
  const space = pathname.match(/^\/spaces\/([^/]+)/)?.[1] ?? 'self';
  const search = new URLSearchParams({ space });
  const revision = params.get('revision');
  if (revision) {
    search.set('revision', revision);
  }
  return (
    <RootProvider
      search={{
        SearchDialog: DefaultSearchDialog,
        options: {
          api: `/api/search?${search}`,
        },
      }}
      i18n={{
        translations: {
          'Search(search dialog)': '搜索文档',
          'Search(search trigger)': '搜索文档',
          'Open Search(search trigger)(aria-label)': '搜索文档',
          'Close Search(search dialog)(aria-label)': '关闭搜索',
          'No results found(search dialog)': '没有找到文档',
          'On this page(table of contents)': '本页目录',
          'Next Page(pagination)': '下一篇',
          'Previous Page(pagination)': '上一篇',
          'Copy Markdown(page actions)': '复制 Markdown',
        },
      }}
    >
      {children}
    </RootProvider>
  );
}
