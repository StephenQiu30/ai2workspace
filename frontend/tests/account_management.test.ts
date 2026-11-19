import assert from 'node:assert/strict';
import {test} from 'node:test';
import {
  canManageAccounts,
  type SessionUser,
} from '../src/features/account/session_state';

test('account management permissions strictly restrict non-admin users', () => {
  const users: Array<{user: SessionUser | null; allowed: boolean}> = [
    {user: null, allowed: false},
    {
      user: {
        userId: 'usr_anon',
        email: 'anon@example.com',
        emailVerified: false,
        roles: [],
        accountStatus: 'active',
      },
      allowed: false,
    },
    {
      user: {
        userId: 'usr_student',
        email: 'student@example.com',
        emailVerified: true,
        roles: ['student'],
        accountStatus: 'active',
      },
      allowed: false,
    },
    {
      user: {
        userId: 'usr_inst',
        email: 'instructor@example.com',
        emailVerified: true,
        roles: ['student', 'instructor'],
        accountStatus: 'active',
      },
      allowed: false,
    },
    {
      user: {
        userId: 'usr_admin',
        email: 'admin@example.com',
        emailVerified: true,
        roles: ['student', 'admin'],
        accountStatus: 'active',
      },
      allowed: true,
    },
  ];

  for (const {user, allowed} of users) {
    assert.equal(
      canManageAccounts(user),
      allowed,
      `Expected canManageAccounts for ${user?.email ?? 'null'} to be ${allowed}`,
    );
  }
});
