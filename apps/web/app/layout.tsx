import type { Metadata } from 'next';
import { headers } from 'next/headers';
import { GeistSans } from 'geist/font/sans';
import { Providers } from '@/components/providers';
import { appName } from '@/lib/shared';
import './global.css';

export const metadata: Metadata = {
  title: { default: appName, template: `%s | ${appName}` },
  description: '本机多空间知识阅读、治理与交接',
};

export default async function Layout({ children }: LayoutProps<'/'>) {
  // Reading request headers keeps the provider dynamic without a shell that
  // commits HTTP 200 before a document can redirect or return not-found.
  await headers();
  return (
    <html lang="zh-CN" className={GeistSans.variable} suppressHydrationWarning>
      <body className="flex min-h-screen flex-col font-sans">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
