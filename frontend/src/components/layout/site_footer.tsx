import {Layout} from '@/components/layout/layout';
import Link from 'next/link';
import {LearnisLogo} from '@/components/brand/learnis_logo';
import {Button} from '@/components/ui/button';

const FOOTER_LINKS = [
  {href: '/', label: '社区'},
  {href: '/courses', label: '课程'},
  {href: '/learning', label: '我的学习'},
  {href: '/orders', label: '订单与退款'},
  {href: '/account', label: '账号'},
];

export function Footer() {
  return (
    <Layout as="footer" className="bg-background">
      <Layout
        flow="stack"
        className="layout-container gap-2 pt-4 pb-6 text-sm text-muted-foreground md:min-h-[84px] md:flex-row md:items-center md:justify-between md:gap-7 md:py-5"
      >
        <LearnisLogo variant="lockup" markSize={22} showEnglish />
        <Layout
          as="nav"
          flow="row"
          aria-label="页脚导航"
          className="flex-wrap items-center gap-x-4 pr-12 md:gap-x-5 md:pr-0"
        >
          {FOOTER_LINKS.map(({href, label}) => (
            <Button
              key={href}
              asChild
              variant="link"
              className="min-h-11 px-0 font-normal"
            >
              <Link href={href}>{label}</Link>
            </Button>
          ))}
        </Layout>
      </Layout>
    </Layout>
  );
}
