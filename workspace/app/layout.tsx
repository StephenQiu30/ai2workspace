import type { Metadata } from 'next';
import { RootProvider } from 'fumadocs-ui/provider/next';
import { GeistSans } from 'geist/font/sans';
import { appName } from '@/lib/shared';
import './global.css';

export const metadata: Metadata = {
  title: { default: appName, template: `%s | ${appName}` },
  description: 'Ai2sd 研发知识库与协作规范',
};

export default function Layout({ children }: LayoutProps<'/'>) {
  return (
    <html lang="zh-CN" className={GeistSans.variable} suppressHydrationWarning>
      <body className="flex min-h-screen flex-col font-sans">
        <RootProvider
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
      </body>
    </html>
  );
}
