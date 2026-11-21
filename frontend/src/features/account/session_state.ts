/** Session user representation retained for legacy state helpers and unit tests. */
export interface SessionUser {
  userId: string;
  email: string;
  emailVerified: boolean;
  roles: string[];
  accountStatus: string;
  nickname?: string;
  version?: number;
}

/** Normalized page modes supported by `/account`. */
export type NormalizedAccountMode =
  'login' | 'register' | 'verify' | 'forgot' | 'reset' | 'invite';

/** Form mode states for user account interactions (compatible with legacy values). */
export type AccountMode =
  NormalizedAccountMode | 'forgot_password' | 'reset_password';

/** Maps query string or legacy mode aliases into canonical `/account` modes. */
export function normalizeAccountMode(
  raw?: string | null,
): NormalizedAccountMode {
  switch (raw?.trim().toLowerCase()) {
    case 'register':
      return 'register';
    case 'verify':
      return 'verify';
    case 'forgot':
    case 'forgot_password':
      return 'forgot';
    case 'reset':
    case 'reset_password':
      return 'reset';
    case 'invite':
      return 'invite';
    default:
      return 'login';
  }
}

/** Validates email format following standard pattern. */
export function validateEmail(email: string): boolean {
  const trimmed = email.trim();
  if (!trimmed || trimmed.length > 254) {
    return false;
  }
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  return emailRegex.test(trimmed);
}

/** Validates password complexity: minimum 8 characters. */
export function validatePassword(password: string): {
  valid: boolean;
  message?: string;
} {
  if (!password || password.length < 8) {
    return {
      valid: false,
      message: '密码长度至少为 8 位字符',
    };
  }
  return {valid: true};
}

/** Checks whether a role list contains a specific role. */
export function hasRole(roles: string[] | undefined, role: string): boolean {
  if (!roles || !Array.isArray(roles)) {
    return false;
  }
  return roles.includes(role);
}

/** Determines whether a user has full access to verified interactions. */
export function canAccessProtectedFeatures(
  user: SessionUser | null | undefined,
): boolean {
  if (!user) {
    return false;
  }
  return user.emailVerified && user.accountStatus === 'active';
}

/** Checks whether a user possesses administrative management privileges. */
export function canManageAccounts(
  user: SessionUser | null | undefined,
): boolean {
  if (!user) {
    return false;
  }
  return hasRole(user.roles, 'admin');
}

/** Checks whether a user is an approved instructor or administrator. */
export function canInstruct(user: SessionUser | null | undefined): boolean {
  if (!user) {
    return false;
  }
  return hasRole(user.roles, 'instructor') || hasRole(user.roles, 'admin');
}

/** Returns Chinese label for an identity role code. */
export function getRoleLabel(role: string): string {
  switch (role) {
    case 'admin':
      return '管理员';
    case 'instructor':
      return '讲师';
    case 'student':
      return '学员';
    default:
      return role;
  }
}

/** Returns Chinese label for account status. */
export function getAccountStatusLabel(status?: string): string {
  switch (status) {
    case 'active':
      return '正常';
    case 'suspended':
      return '已停用';
    default:
      return status || '未知';
  }
}

/** Returns Chinese label for instructor invitation status. */
export function getInvitationStatusLabel(status?: string): string {
  switch (status) {
    case 'pending':
      return '待接受';
    case 'accepted':
      return '已接受';
    case 'cancelled':
      return '已撤销';
    case 'expired':
      return '已过期';
    default:
      return status || '未知';
  }
}
