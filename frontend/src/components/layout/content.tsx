import {Layout} from '@/components/layout/layout';
import type {ReactNode} from 'react';

export function Content({children}: {children: ReactNode}) {
  return (
    <Layout
      as="main"
      id="main-content"
      tabIndex={-1}
      className="layout-container min-w-0 flex-1 pt-4 pb-12 md:pt-6 md:pb-16"
    >
      {children}
    </Layout>
  );
}
