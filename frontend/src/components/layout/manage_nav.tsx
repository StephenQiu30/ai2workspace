'use client';

import {Layout} from '@/components/layout/layout';
import Link from 'next/link';
import {usePathname} from 'next/navigation';
import {Button} from '@/components/ui/button';
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
    <Layout
      as="nav"
      flow="row"
      aria-label="管理区域导航"
      className="mb-8 flex-wrap items-center gap-1.5"
    >
      {visibleItems.map((item) => {
        const active =
          pathname === item.href || pathname.startsWith(`${item.href}/`);
        return (
          <Button
            key={item.href}
            asChild
            size="sm"
            variant={active ? 'default' : 'secondary'}
          >
            <Link href={item.href} aria-current={active ? 'page' : undefined}>
              {item.label}
            </Link>
          </Button>
        );
      })}
    </Layout>
  );
}
