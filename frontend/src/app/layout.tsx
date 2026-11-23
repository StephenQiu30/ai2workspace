import type {Metadata} from 'next';
import type {ReactNode} from 'react';
import {Geist, Geist_Mono} from 'next/font/google';
import {BasicLayout} from '@/components/layout/basic_layout';
import {Toaster} from '@/components/ui/sonner';
import {TooltipProvider} from '@/components/ui/tooltip';
import {AssistantProvider} from '@/features/assistant/assistant_context';
import {FloatingAssistant} from '@/features/assistant/floating_assistant';
import {getServerSession} from '@/lib/auth/session';
import {brandFullTitle, projectDescription} from '@/lib/project';
import './globals.css';

const geistSans = Geist({
  variable: '--font-geist-sans',
  subsets: ['latin'],
});

const geistMono = Geist_Mono({
  variable: '--font-geist-mono',
  subsets: ['latin'],
});

export const metadata: Metadata = {
  title: {
    default: `${brandFullTitle} · 课程学习与交流社区`,
    template: `%s · ${brandFullTitle}`,
  },
  description: projectDescription,
};

export default async function RootLayout({
  children,
}: Readonly<{
  children: ReactNode;
}>) {
  const user = await getServerSession();

  return (
    <html
      lang="zh-CN"
      className={`${geistSans.variable} ${geistMono.variable}`}
    >
      <body className="min-h-screen bg-background font-sans text-foreground antialiased">
        <TooltipProvider>
          <AssistantProvider>
            <BasicLayout user={user}>{children}</BasicLayout>
            <FloatingAssistant initialUser={user} />
            <Toaster position="top-center" />
          </AssistantProvider>
        </TooltipProvider>
      </body>
    </html>
  );
}
