import type {Metadata} from 'next';
import type {ReactNode} from 'react';
import {Geist, Geist_Mono} from 'next/font/google';
import {SiteFooter} from '@/components/layout/site_footer';
import {SiteHeader} from '@/components/layout/site_header';
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
            <div className="flex min-h-screen flex-col">
              <SiteHeader initialUser={user} />
              <main className="mx-auto w-full max-w-[1920px] flex-1 px-4 pt-6 pb-[104px] md:px-6 md:pt-10 xl:px-16">
                {children}
              </main>
              <SiteFooter />
            </div>
            <FloatingAssistant initialUser={user} />
            <Toaster position="top-center" />
          </AssistantProvider>
        </TooltipProvider>
      </body>
    </html>
  );
}
