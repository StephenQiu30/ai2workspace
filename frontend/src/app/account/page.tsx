import type {Metadata} from 'next';
import {AccountForm} from '@/features/account/account_form';
import type {SessionUser} from '@/features/account/session_state';
import {getServerRequestOptions} from '@/lib/request/server';
import {identityGetCurrentUser} from '@/services/generated/identity/sessions';

export const metadata: Metadata = {
  title: '账号与个人中心 · ai2workspace',
  description: '登录、注册与管理您的学员账号及凭据。',
};

export const dynamic = 'force-dynamic';

export default async function AccountPage() {
  let initialUser: SessionUser | null = null;

  try {
    const options = await getServerRequestOptions();
    const currentUser = await identityGetCurrentUser(options);
    if (currentUser && currentUser.userId) {
      initialUser = {
        userId: currentUser.userId,
        email: currentUser.email ?? '',
        emailVerified: currentUser.emailVerified ?? false,
        roles: currentUser.roles ?? ['student'],
        accountStatus: currentUser.accountStatus ?? 'active',
      };
    }
  } catch {
    // Unauthenticated or service unavailable defaults to null initial user.
  }

  return (
    <main className="mx-auto flex min-h-svh w-full max-w-xl flex-col justify-center px-4 py-12 sm:px-6">
      <div className="mb-8 text-center">
        <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">
          ai2workspace 账号服务
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          提供可信身份认证、邮箱验证与权限管理
        </p>
      </div>
      <AccountForm initialUser={initialUser} />
    </main>
  );
}
