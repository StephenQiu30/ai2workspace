import type {Metadata} from 'next';
import {AccountManagement} from '@/features/account/account_management';
import type {SessionUser} from '@/features/account/session_state';
import {getServerRequestOptions} from '@/lib/request/server';
import {identityGetCurrentUser} from '@/services/generated/identity/sessions';

export const metadata: Metadata = {
  title: '账号与权限管理 · ai2workspace',
  description: '管理员控制台：受控讲师邀请、资质撤销及账号状态处置。',
};

export const dynamic = 'force-dynamic';

export default async function ManageAccountsPage() {
  let currentUser: SessionUser | null = null;

  try {
    const options = await getServerRequestOptions();
    const user = await identityGetCurrentUser(options);
    if (user && user.userId) {
      currentUser = {
        userId: user.userId,
        email: user.email ?? '',
        emailVerified: user.emailVerified ?? false,
        roles: user.roles ?? ['student'],
        accountStatus: user.accountStatus ?? 'active',
      };
    }
  } catch {
    // Unauthenticated user will see access restricted message in the management component
  }

  return (
    <main className="mx-auto flex min-h-svh w-full max-w-4xl flex-col px-4 py-12 sm:px-6">
      <div className="mb-8">
        <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">
          账号与权限治理
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          受控讲师邀请、资格撤销及账号停用处置（需具备管理员身份）
        </p>
      </div>
      <AccountManagement currentUser={currentUser} />
    </main>
  );
}
