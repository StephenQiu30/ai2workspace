'use client';
import SearchDialog from '@/components/search';
import { RootProvider } from 'fumadocs-ui/provider/next';
import { type ReactNode } from 'react';

export function Provider({ children }: { children: ReactNode }) {
  return (
    <RootProvider
      search={{ SearchDialog }}
      i18n={{
        translations: {
          'Search(search trigger)': '搜索文档',
          'Open Search(search trigger)(aria-label)': '搜索文档',
          'Search(search dialog)': '搜索文档',
          'No results found(search dialog)': '没有找到相关文档',
          'Close Search(search dialog)(aria-label)': '关闭搜索',
          'On this page(table of contents)': '本页目录',
          'Next Page(pagination)': '下一页',
          'Previous Page(pagination)': '上一页',
          'Open Sidebar(sidebar)(aria-label)': '打开目录',
          'Close Sidebar(sidebar)(aria-label)': '关闭目录',
        },
      }}
    >
      {children}
    </RootProvider>
  );
}
