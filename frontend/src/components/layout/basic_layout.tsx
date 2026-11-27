import {Layout} from '@/components/layout/layout';
import type {ReactNode} from 'react';
import {Content} from '@/components/layout/content';
import {Footer} from '@/components/layout/site_footer';
import {Navbar} from '@/components/layout/site_header';
import {Button} from '@/components/ui/button';
import type {SessionUser} from '@/lib/auth/roles';

export function BasicLayout({
  children,
  user,
}: {
  children: ReactNode;
  user: SessionUser | null;
}) {
  return (
    <Layout flow="stack" className="min-h-dvh">
      <Button
        asChild
        className="sr-only focus-visible:not-sr-only focus-visible:fixed focus-visible:top-2 focus-visible:left-2 focus-visible:z-50"
      >
        <Layout as="a" href="#main-content">
          跳至正文
        </Layout>
      </Button>
      <Navbar initialUser={user} />
      <Content>{children}</Content>
      <Footer />
    </Layout>
  );
}
