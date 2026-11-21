'use client';

import Link from 'next/link';
import {usePathname} from 'next/navigation';
import {cn} from 'cn';
import {
  canManageCourses,
  canManageKnowledge,
  isAdmin,
  type SessionUser,
} from '@/lib/auth/roles';

export interface ManageNavProps {
  user: SessionUser | null;
}

interface NavItem {
  href: string;
  label: string;
  visible: (user: SessionUser | null) => boolean;
}

const MANAGE_ITEMS: readonly NavItem[] = [
  {href: '/manage/courses', label: '课程管理', visible: canManageCourses},
  {href: '/manage/knowledge', label: '知识库管理', visible: canManageKnowledge},
  {href: '/manage/accounts', label: '账号管理', visible: isAdmin},
  {href: '/manage/refunds', label: '退款处理', visible: isAdmin},
  {href: '/manage/reports', label: '内容举报', visible: isAdmin},
  {href: '/manage/agents', label: 'Agent 管理', visible: isAdmin},
  {href: '/manage/ai-usage', label: '模型与费用', visible: isAdmin},
];

export function ManageNav({user}: ManageNavProps) {
  const pathname = usePathname() || '';
  const visibleItems = MANAGE_ITEMS.filter((item) => item.visible(user));

  if (visibleItems.length === 0) {
    return null;
  }

  return (
    <nav
      aria-label="管理区域导航"
      className="mb-8 flex flex-wrap items-center gap-1.5"
    >
      {visibleItems.map((item) => {
        const active =
          pathname === item.href || pathname.startsWith(`${item.href}/`);
        return (
          <Link
            key={item.href}
            href={item.href}
            aria-current={active ? 'page' : undefined}
            className={cn(
              'inline-flex h-9 items-center rounded-md px-3 text-sm font-medium transition-colors duration-120 outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
              active
                ? 'bg-primary text-primary-foreground'
                : 'bg-secondary text-secondary-foreground hover:bg-control-hover',
            )}
          >
            {item.label}
          </Link>
        );
      })}
    </nav>
  );
}
