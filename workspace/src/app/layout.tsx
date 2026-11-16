import type { Metadata } from 'next';
import { Geist, JetBrains_Mono } from 'next/font/google';
import { appName } from '@/lib/shared';
import { Provider } from '@/components/provider';
import './global.css';

const geist = Geist({
  variable: '--font-sans',
  subsets: ['latin'],
});

const mono = JetBrains_Mono({
  variable: '--font-mono',
  subsets: ['latin'],
});

export const metadata: Metadata = {
  title: { default: appName, template: `%s | ${appName}` },
  description: 'Ai2Workspace 项目知识库',
};

export default function Layout({ children }: LayoutProps<'/'>) {
  return (
    <html lang="zh-CN" className={`${geist.variable} ${mono.variable}`} suppressHydrationWarning>
      <body className="flex flex-col min-h-screen">
        <Provider>{children}</Provider>
      </body>
    </html>
  );
}
