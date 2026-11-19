/** Session user representation retrieved from the identity service. */
export interface SessionUser {
  userId: string;
  email: string;
  emailVerified: boolean;
  roles: string[];
  accountStatus: string;
}

/** Form mode states for user account interactions. */
export type AccountMode =
  'login' | 'register' | 'verify' | 'forgot_password' | 'reset_password';

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
