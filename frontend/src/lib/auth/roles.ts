export interface SessionUser {
  id: string;
  email: string;
  nickname: string;
  emailVerified: boolean;
  status: string;
  roles: string[];
  version: number;
}

export function normalizeSessionUser(
  raw: IdentityApi.CurrentUserResponse | null | undefined,
): SessionUser | null {
  if (!raw) {
    return null;
  }
  const id = raw.id ?? raw.userId ?? '';
  if (!id) {
    return null;
  }
  const email = raw.email ?? '';
  const fallbackNickname = email.includes('@') ? email.split('@')[0] : id;
  return {
    id,
    email,
    nickname: raw.nickname?.trim() || fallbackNickname,
    emailVerified: Boolean(raw.emailVerified),
    status: raw.status ?? raw.accountStatus ?? 'active',
    roles: Array.isArray(raw.roles) ? raw.roles : ['student'],
    version: raw.version ?? 1,
  };
}

export function isAdmin(user: SessionUser | null | undefined): boolean {
  return Boolean(user?.roles.includes('admin'));
}

export function isInstructor(user: SessionUser | null | undefined): boolean {
  return Boolean(user?.roles.includes('instructor'));
}

export function canManageCourses(
  user: SessionUser | null | undefined,
): boolean {
  return isAdmin(user) || isInstructor(user);
}

export function canManageKnowledge(
  user: SessionUser | null | undefined,
): boolean {
  return isAdmin(user) || isInstructor(user);
}

export function canManageAgents(user: SessionUser | null | undefined): boolean {
  return isAdmin(user);
}

export function getUserInitials(user: SessionUser): string {
  const label = (user.nickname || user.email || '学').trim();
  return label.slice(0, 1).toUpperCase();
}
