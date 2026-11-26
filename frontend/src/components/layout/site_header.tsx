'use client';

import {Layout} from '@/components/layout/layout';
import {Text} from '@/components/layout/typography';
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
import {identityLogout} from '@/services/generated/api/sessions';

interface NavbarProps {
  initialUser: SessionUser | null;
}

const PRIMARY_NAV = [
  {
    href: '/',
    label: '社区',
    match: (p: string) =>
      p === '/' ||
      p.startsWith('/community') ||
      (p.startsWith('/topics') && p !== '/topics/services'),
  },
  {
    href: '/courses',
    label: '课程',
    match: (p: string) => p.startsWith('/courses'),
  },
  {
    href: '/topics/services',
    label: '线上服务',
    match: (p: string) => p === '/topics/services',
  },
  {
    href: '/learning',
    label: '我的学习',
    match: (p: string) => p.startsWith('/learning') || p.startsWith('/learn/'),
  },
] as const;

function PrimaryNavigation({
  pathname,
  user,
  onNavigate,
}: {
  pathname: string;
  user: SessionUser | null;
  onNavigate?: () => void;
}) {
  const mobile = Boolean(onNavigate);
  return (
    <NavigationMenu
      viewport={false}
      aria-label={mobile ? '移动主导航' : '主导航'}
      orientation={mobile ? 'vertical' : 'horizontal'}
      className={
        mobile
          ? 'w-full max-w-none flex-none justify-start [&>div]:w-full'
          : 'hidden flex-none justify-start md:flex'
      }
    >
      <NavigationMenuList
        className={
          mobile ? 'w-full flex-col items-stretch gap-1' : 'justify-start gap-1'
        }
      >
        {PRIMARY_NAV.map((item) => {
          const active = item.match(pathname);
          return (
            <NavigationMenuItem key={item.href}>
              <NavigationMenuLink
                asChild
                active={active}
                className={
                  mobile
                    ? 'min-h-11 justify-center px-3'
                    : 'min-h-10 justify-center px-3.5'
                }
              >
                <Link
                  href={
                    item.href === '/learning' && !user
                      ? buildLoginHref('/learning')
                      : item.href
                  }
                  aria-current={active ? 'page' : undefined}
                  onNavigate={onNavigate}
                >
                  {item.label}
                </Link>
              </NavigationMenuLink>
            </NavigationMenuItem>
          );
        })}
      </NavigationMenuList>
    </NavigationMenu>
  );
}

export function Navbar({initialUser}: NavbarProps) {
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
    <Layout
      as="header"
      className="layout-container sticky top-0 z-40 bg-background/95 backdrop-blur-xs"
    >
      <Layout
        flow="row"
        className="h-16 w-full items-center gap-4 md:h-[68px] md:gap-6"
      >
        <Link
          href="/"
          aria-label="学屿首页"
          className="inline-flex w-fit items-center rounded-md py-1 outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
        >
          <LearnisLogo variant="lockup" markSize={28} />
        </Link>

        <PrimaryNavigation pathname={pathname} user={user} />

        <Layout flow="row" className="ml-auto items-center gap-2">
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
                    <Text className="truncate text-sm font-medium text-foreground">
                      {user.nickname}
                    </Text>
                    <Text className="truncate text-xs font-normal text-muted-foreground">
                      {user.email}
                    </Text>
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
            <Layout className="hidden items-center gap-2 md:flex">
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
            </Layout>
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
              <Layout flow="stack" className="gap-1 px-4">
                <PrimaryNavigation
                  pathname={pathname}
                  user={user}
                  onNavigate={() => setMobileOpen(false)}
                />
                {user ? (
                  <>
                    <Layout className="mt-4 px-3 text-xs font-medium text-muted-foreground">
                      个人空间
                    </Layout>
                    <SheetClose asChild>
                      <Button
                        asChild
                        variant="ghost"
                        className="min-h-11 justify-start"
                      >
                        <Link href="/account">本人账号</Link>
                      </Button>
                    </SheetClose>
                    <SheetClose asChild>
                      <Button
                        asChild
                        variant="ghost"
                        className="min-h-11 justify-start"
                      >
                        <Link href="/orders">我的订单</Link>
                      </Button>
                    </SheetClose>
                    <SheetClose asChild>
                      <Button
                        asChild
                        variant="ghost"
                        className="min-h-11 justify-start"
                      >
                        <Link href="/notifications">站内通知</Link>
                      </Button>
                    </SheetClose>
                    {hasAnyManageAccess ? (
                      <>
                        <Layout className="mt-4 px-3 text-xs font-medium text-muted-foreground">
                          管理工作台
                        </Layout>
                        {courseManager ? (
                          <SheetClose asChild>
                            <Button
                              asChild
                              variant="ghost"
                              className="min-h-11 justify-start"
                            >
                              <Link href="/manage/courses">课程管理</Link>
                            </Button>
                          </SheetClose>
                        ) : null}
                        {knowledgeManager ? (
                          <SheetClose asChild>
                            <Button
                              asChild
                              variant="ghost"
                              className="min-h-11 justify-start"
                            >
                              <Link href="/manage/knowledge">知识库管理</Link>
                            </Button>
                          </SheetClose>
                        ) : null}
                        {adminUser ? (
                          <>
                            <SheetClose asChild>
                              <Button
                                asChild
                                variant="ghost"
                                className="min-h-11 justify-start"
                              >
                                <Link href="/manage/accounts">账号管理</Link>
                              </Button>
                            </SheetClose>
                            <SheetClose asChild>
                              <Button
                                asChild
                                variant="ghost"
                                className="min-h-11 justify-start"
                              >
                                <Link href="/manage/refunds">退款处理</Link>
                              </Button>
                            </SheetClose>
                            <SheetClose asChild>
                              <Button
                                asChild
                                variant="ghost"
                                className="min-h-11 justify-start"
                              >
                                <Link href="/manage/reports">内容举报</Link>
                              </Button>
                            </SheetClose>
                            <SheetClose asChild>
                              <Button
                                asChild
                                variant="ghost"
                                className="min-h-11 justify-start"
                              >
                                <Link href="/manage/agents">Agent 管理</Link>
                              </Button>
                            </SheetClose>
                            <SheetClose asChild>
                              <Button
                                asChild
                                variant="ghost"
                                className="min-h-11 justify-start"
                              >
                                <Link href="/manage/ai-usage">模型与费用</Link>
                              </Button>
                            </SheetClose>
                          </>
                        ) : null}
                      </>
                    ) : null}
                    <Layout className="mt-4 px-3">
                      <Button
                        variant="secondary"
                        className="w-full"
                        disabled={loggingOut}
                        onClick={() => void handleLogout()}
                      >
                        {loggingOut ? '正在退出…' : '退出登录'}
                      </Button>
                    </Layout>
                  </>
                ) : (
                  <Layout flow="stack" className="mt-4 gap-2 px-3">
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
                  </Layout>
                )}
              </Layout>
            </SheetContent>
          </Sheet>
        </Layout>
      </Layout>
    </Layout>
  );
}
