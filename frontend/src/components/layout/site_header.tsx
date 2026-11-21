'use client';

import {useState} from 'react';
import Link from 'next/link';
import {usePathname, useRouter} from 'next/navigation';
import {
  BellIcon,
  LogOutIcon,
  MenuIcon,
  ReceiptTextIcon,
  Settings2Icon,
  UserIcon,
} from 'lucide-react';
import {cn} from 'cn';
import {toast} from 'sonner';
import {LearnisLogo} from '@/components/brand/learnis_logo';
import {Avatar, AvatarFallback} from '@/components/ui/avatar';
import {Button} from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  NavigationMenu,
  NavigationMenuItem,
  NavigationMenuLink,
  NavigationMenuList,
} from '@/components/ui/navigation-menu';
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet';
import {
  canManageCourses,
  canManageKnowledge,
  getUserInitials,
  isAdmin,
  type SessionUser,
} from '@/lib/auth/roles';
import {buildLoginHref, getErrorMessage} from '@/lib/format';
import {identityLogout} from '@/services/generated/identity/sessions';

export interface SiteHeaderProps {
  initialUser: SessionUser | null;
}

const PRIMARY_NAV = [
  {
    href: '/',
    label: '社区',
    match: (p: string) =>
      p === '/' || p.startsWith('/community') || p.startsWith('/topics'),
  },
  {
    href: '/courses',
    label: '课程',
    match: (p: string) => p.startsWith('/courses'),
  },
  {
    href: '/learning',
    label: '我的学习',
    match: (p: string) => p.startsWith('/learning') || p.startsWith('/learn/'),
  },
] as const;

export function SiteHeader({initialUser}: SiteHeaderProps) {
  const pathname = usePathname() || '/';
  const router = useRouter();
  const [user, setUser] = useState<SessionUser | null>(initialUser);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [loggingOut, setLoggingOut] = useState(false);

  const adminUser = isAdmin(user);
  const courseManager = canManageCourses(user);
  const knowledgeManager = canManageKnowledge(user);
  const hasAnyManageAccess = adminUser || courseManager || knowledgeManager;

  async function handleLogout() {
    setLoggingOut(true);
    try {
      await identityLogout();
      setUser(null);
      setMobileOpen(false);
      toast.success('已安全退出登录');
      router.push('/');
      router.refresh();
    } catch (err: unknown) {
      toast.error(getErrorMessage(err, '退出登录失败'));
    } finally {
      setLoggingOut(false);
    }
  }

  return (
    <header className="sticky top-0 z-40 w-full bg-background/95 backdrop-blur-xs">
      <div className="mx-auto flex h-16 max-w-[1920px] items-center justify-between gap-4 px-4 md:h-20 md:px-6 xl:px-16">
        <div className="flex items-center gap-8">
          <Link
            href="/"
            aria-label="学屿首页"
            className="inline-flex items-center rounded-md py-1 outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            <LearnisLogo variant="lockup" markSize={28} />
          </Link>

          <NavigationMenu className="hidden md:flex">
            <NavigationMenuList className="gap-1">
              {PRIMARY_NAV.map((item) => {
                const active = item.match(pathname);
                const targetHref =
                  item.href === '/learning' && !user
                    ? buildLoginHref('/learning')
                    : item.href;
                return (
                  <NavigationMenuItem key={item.href}>
                    <NavigationMenuLink asChild>
                      <Link
                        href={targetHref}
                        aria-current={active ? 'page' : undefined}
                        className={cn(
                          'inline-flex h-10 items-center rounded-md px-3.5 text-[14px] font-medium transition-colors duration-120 outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
                          active
                            ? 'bg-secondary text-foreground'
                            : 'text-muted-foreground hover:bg-canvas-soft hover:text-foreground',
                        )}
                      >
                        {item.label}
                      </Link>
                    </NavigationMenuLink>
                  </NavigationMenuItem>
                );
              })}
            </NavigationMenuList>
          </NavigationMenu>
        </div>

        <div className="flex items-center gap-2">
          {user ? (
            <>
              <Button
                variant={
                  pathname.startsWith('/notifications') ? 'secondary' : 'ghost'
                }
                size="icon"
                asChild
                aria-label="站内通知"
              >
                <Link href="/notifications">
                  <BellIcon />
                </Link>
              </Button>

              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="rounded-full"
                    aria-label={`打开 ${user.nickname} 的账号菜单`}
                  >
                    <Avatar className="size-9">
                      <AvatarFallback className="bg-primary text-xs font-medium text-primary-foreground">
                        {getUserInitials(user)}
                      </AvatarFallback>
                    </Avatar>
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-56">
                  <DropdownMenuLabel className="flex flex-col gap-0.5 py-2">
                    <span className="truncate text-sm font-medium text-foreground">
                      {user.nickname}
                    </span>
                    <span className="truncate text-xs font-normal text-muted-foreground">
                      {user.email}
                    </span>
                  </DropdownMenuLabel>
                  <DropdownMenuGroup>
                    <DropdownMenuItem asChild>
                      <Link href="/account">
                        <UserIcon />
                        本人账号与安全
                      </Link>
                    </DropdownMenuItem>
                    <DropdownMenuItem asChild>
                      <Link href="/orders">
                        <ReceiptTextIcon />
                        我的订单与售后
                      </Link>
                    </DropdownMenuItem>
                  </DropdownMenuGroup>

                  {hasAnyManageAccess ? (
                    <>
                      <DropdownMenuSeparator />
                      <DropdownMenuLabel>管理工作台</DropdownMenuLabel>
                      <DropdownMenuGroup>
                        {courseManager ? (
                          <DropdownMenuItem asChild>
                            <Link href="/manage/courses">
                              <Settings2Icon />
                              课程管理
                            </Link>
                          </DropdownMenuItem>
                        ) : null}
                        {knowledgeManager ? (
                          <DropdownMenuItem asChild>
                            <Link href="/manage/knowledge">
                              <Settings2Icon />
                              知识库管理
                            </Link>
                          </DropdownMenuItem>
                        ) : null}
                        {adminUser ? (
                          <>
                            <DropdownMenuItem asChild>
                              <Link href="/manage/accounts">账号管理</Link>
                            </DropdownMenuItem>
                            <DropdownMenuItem asChild>
                              <Link href="/manage/refunds">退款处理</Link>
                            </DropdownMenuItem>
                            <DropdownMenuItem asChild>
                              <Link href="/manage/reports">内容举报</Link>
                            </DropdownMenuItem>
                            <DropdownMenuItem asChild>
                              <Link href="/manage/agents">Agent 管理</Link>
                            </DropdownMenuItem>
                            <DropdownMenuItem asChild>
                              <Link href="/manage/ai-usage">模型与费用</Link>
                            </DropdownMenuItem>
                          </>
                        ) : null}
                      </DropdownMenuGroup>
                    </>
                  ) : null}

                  <DropdownMenuSeparator />
                  <DropdownMenuGroup>
                    <DropdownMenuItem
                      disabled={loggingOut}
                      onSelect={(e) => {
                        e.preventDefault();
                        void handleLogout();
                      }}
                    >
                      <LogOutIcon />
                      {loggingOut ? '正在退出…' : '退出登录'}
                    </DropdownMenuItem>
                  </DropdownMenuGroup>
                </DropdownMenuContent>
              </DropdownMenu>
            </>
          ) : (
            <div className="hidden items-center gap-2 md:flex">
              <Button variant="ghost" asChild>
                <Link href={buildLoginHref(pathname)}>登录</Link>
              </Button>
              <Button asChild>
                <Link
                  href={`${buildLoginHref(pathname)}${buildLoginHref(pathname).includes('?') ? '&' : '?'}mode=register`}
                >
                  注册账号
                </Link>
              </Button>
            </div>
          )}

          <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
            <SheetTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="md:hidden"
                aria-label="打开导航菜单"
              >
                <MenuIcon />
              </Button>
            </SheetTrigger>
            <SheetContent side="left" className="w-80">
              <SheetHeader>
                <SheetTitle>
                  <LearnisLogo variant="lockup" markSize={24} />
                </SheetTitle>
              </SheetHeader>
              <nav aria-label="移动主导航" className="flex flex-col gap-1 px-4">
                {PRIMARY_NAV.map((item) => {
                  const active = item.match(pathname);
                  const targetHref =
                    item.href === '/learning' && !user
                      ? buildLoginHref('/learning')
                      : item.href;
                  return (
                    <SheetClose asChild key={item.href}>
                      <Link
                        href={targetHref}
                        aria-current={active ? 'page' : undefined}
                        className={cn(
                          'flex h-11 items-center rounded-md px-3 text-sm font-medium',
                          active
                            ? 'bg-secondary text-foreground'
                            : 'text-body hover:bg-canvas-soft hover:text-foreground',
                        )}
                      >
                        {item.label}
                      </Link>
                    </SheetClose>
                  );
                })}
                {user ? (
                  <>
                    <div className="mt-4 px-3 text-xs font-medium text-muted-foreground">
                      个人空间
                    </div>
                    <SheetClose asChild>
                      <Link
                        href="/account"
                        className="flex h-11 items-center rounded-md px-3 text-sm text-body hover:bg-canvas-soft hover:text-foreground"
                      >
                        本人账号
                      </Link>
                    </SheetClose>
                    <SheetClose asChild>
                      <Link
                        href="/orders"
                        className="flex h-11 items-center rounded-md px-3 text-sm text-body hover:bg-canvas-soft hover:text-foreground"
                      >
                        我的订单
                      </Link>
                    </SheetClose>
                    <SheetClose asChild>
                      <Link
                        href="/notifications"
                        className="flex h-11 items-center rounded-md px-3 text-sm text-body hover:bg-canvas-soft hover:text-foreground"
                      >
                        站内通知
                      </Link>
                    </SheetClose>
                    {hasAnyManageAccess ? (
                      <>
                        <div className="mt-4 px-3 text-xs font-medium text-muted-foreground">
                          管理工作台
                        </div>
                        {courseManager ? (
                          <SheetClose asChild>
                            <Link
                              href="/manage/courses"
                              className="flex h-11 items-center rounded-md px-3 text-sm text-body hover:bg-canvas-soft hover:text-foreground"
                            >
                              课程管理
                            </Link>
                          </SheetClose>
                        ) : null}
                        {knowledgeManager ? (
                          <SheetClose asChild>
                            <Link
                              href="/manage/knowledge"
                              className="flex h-11 items-center rounded-md px-3 text-sm text-body hover:bg-canvas-soft hover:text-foreground"
                            >
                              知识库管理
                            </Link>
                          </SheetClose>
                        ) : null}
                        {adminUser ? (
                          <>
                            <SheetClose asChild>
                              <Link
                                href="/manage/accounts"
                                className="flex h-11 items-center rounded-md px-3 text-sm text-body hover:bg-canvas-soft hover:text-foreground"
                              >
                                账号管理
                              </Link>
                            </SheetClose>
                            <SheetClose asChild>
                              <Link
                                href="/manage/refunds"
                                className="flex h-11 items-center rounded-md px-3 text-sm text-body hover:bg-canvas-soft hover:text-foreground"
                              >
                                退款处理
                              </Link>
                            </SheetClose>
                            <SheetClose asChild>
                              <Link
                                href="/manage/reports"
                                className="flex h-11 items-center rounded-md px-3 text-sm text-body hover:bg-canvas-soft hover:text-foreground"
                              >
                                内容举报
                              </Link>
                            </SheetClose>
                            <SheetClose asChild>
                              <Link
                                href="/manage/agents"
                                className="flex h-11 items-center rounded-md px-3 text-sm text-body hover:bg-canvas-soft hover:text-foreground"
                              >
                                Agent 管理
                              </Link>
                            </SheetClose>
                            <SheetClose asChild>
                              <Link
                                href="/manage/ai-usage"
                                className="flex h-11 items-center rounded-md px-3 text-sm text-body hover:bg-canvas-soft hover:text-foreground"
                              >
                                模型与费用
                              </Link>
                            </SheetClose>
                          </>
                        ) : null}
                      </>
                    ) : null}
                    <div className="mt-4 px-3">
                      <Button
                        variant="secondary"
                        className="w-full"
                        disabled={loggingOut}
                        onClick={() => void handleLogout()}
                      >
                        {loggingOut ? '正在退出…' : '退出登录'}
                      </Button>
                    </div>
                  </>
                ) : (
                  <div className="mt-4 flex flex-col gap-2 px-3">
                    <SheetClose asChild>
                      <Button asChild className="w-full">
                        <Link href={buildLoginHref(pathname)}>登录账号</Link>
                      </Button>
                    </SheetClose>
                    <SheetClose asChild>
                      <Button variant="secondary" asChild className="w-full">
                        <Link href="/account?mode=register">注册新账号</Link>
                      </Button>
                    </SheetClose>
                  </div>
                )}
              </nav>
            </SheetContent>
          </Sheet>
        </div>
      </div>
    </header>
  );
}
