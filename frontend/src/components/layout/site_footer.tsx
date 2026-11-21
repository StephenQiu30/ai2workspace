import Link from 'next/link';
import {LearnisLogo} from '@/components/brand/learnis_logo';

export function SiteFooter() {
  return (
    <footer className="mt-16 w-full bg-background">
      <div className="mx-auto flex min-h-[104px] max-w-[1920px] flex-col justify-between gap-6 px-4 py-8 text-sm text-muted-foreground md:flex-row md:items-center md:px-6 xl:px-16">
        <div className="flex flex-col gap-1.5">
          <LearnisLogo variant="lockup" markSize={22} showEnglish />
          <p className="text-xs leading-5 text-muted-foreground">
            课程学习、交流社区与有来源的通用知识问答平台。
          </p>
        </div>

        <nav
          aria-label="页脚导航"
          className="flex flex-wrap items-center gap-x-6 gap-y-2 text-sm"
        >
          <Link
            href="/"
            className="text-body underline-offset-4 hover:text-foreground hover:underline"
          >
            社区首页
          </Link>
          <Link
            href="/courses"
            className="text-body underline-offset-4 hover:text-foreground hover:underline"
          >
            课程目录
          </Link>
          <Link
            href="/learning"
            className="text-body underline-offset-4 hover:text-foreground hover:underline"
          >
            我的学习
          </Link>
          <Link
            href="/orders"
            className="text-body underline-offset-4 hover:text-foreground hover:underline"
          >
            订单与退款规则
          </Link>
          <Link
            href="/account"
            className="text-body underline-offset-4 hover:text-foreground hover:underline"
          >
            账号与安全
          </Link>
        </nav>
      </div>
    </footer>
  );
}
