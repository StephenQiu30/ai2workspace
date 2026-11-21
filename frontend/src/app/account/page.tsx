import type {Metadata} from 'next';
import {AccountForm} from '@/features/account/account_form';
import {normalizeAccountMode} from '@/features/account/session_state';
import {getServerSession} from '@/lib/auth/session';
import {sanitizeNextPath} from '@/lib/format';

export const metadata: Metadata = {
  title: '账号与认证',
  description: '登录、注册、邮箱验证、密码重置与个人资料管理。',
};

export const dynamic = 'force-dynamic';

interface AccountPageProps {
  searchParams: Promise<{
    next?: string;
    mode?: string;
    token?: string;
  }>;
}

export default async function AccountPage({searchParams}: AccountPageProps) {
  const [user, params] = await Promise.all([getServerSession(), searchParams]);
  const safeNext = sanitizeNextPath(params.next);
  const initialMode = normalizeAccountMode(params.mode);
  const initialToken = typeof params.token === 'string' ? params.token : '';

  return (
    <section className="mx-auto w-full max-w-2xl space-y-6">
      <header className="space-y-1.5">
        <h1 className="text-[28px] leading-[40px] font-medium text-foreground">
          {user ? '本人账号与安全' : '账号与认证'}
        </h1>
        <p className="text-sm leading-6 text-muted-foreground">
          {user
            ? '管理公开昵称、邮箱验证状态、讲师邀请资格与快捷学习入口。'
            : '登录或注册学屿账号，验证邮箱后即可参与社区讨论与课程学习。'}
        </p>
      </header>

      <AccountForm
        initialUser={user}
        nextPath={safeNext}
        initialMode={initialMode}
        initialToken={initialToken}
      />
    </section>
  );
}
