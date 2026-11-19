import assert from 'node:assert/strict';
import {test} from 'node:test';
import {
  canAccessProtectedFeatures,
  canInstruct,
  canManageAccounts,
  hasRole,
  type SessionUser,
  validateEmail,
  validatePassword,
} from '../src/features/account/session_state';

test('validateEmail correctly handles valid and invalid email formats', () => {
  assert.equal(validateEmail('test@example.com'), true);
  assert.equal(validateEmail('first.last+tag@sub.domain.org'), true);

  assert.equal(validateEmail(''), false);
  assert.equal(validateEmail('not-an-email'), false);
  assert.equal(validateEmail('@domain.com'), false);
  assert.equal(validateEmail('user@'), false);
  assert.equal(validateEmail('user@domain'), false);
});

test('validatePassword requires at least 8 characters', () => {
  assert.equal(validatePassword('12345678').valid, true);
  assert.equal(validatePassword('StrongPass123!').valid, true);

  assert.equal(validatePassword('').valid, false);
  assert.equal(validatePassword('1234567').valid, false);
});

test('hasRole accurately checks role presence', () => {
  assert.equal(hasRole(['student', 'instructor'], 'instructor'), true);
  assert.equal(hasRole(['student'], 'admin'), false);
  assert.equal(hasRole([], 'student'), false);
  assert.equal(hasRole(undefined, 'student'), false);
});

test('canAccessProtectedFeatures enforces verified and active account status', () => {
  const verifiedActive: SessionUser = {
    userId: 'usr_1',
    email: 'user@example.com',
    emailVerified: true,
    roles: ['student'],
    accountStatus: 'active',
  };
  assert.equal(canAccessProtectedFeatures(verifiedActive), true);

  const unverified: SessionUser = {
    ...verifiedActive,
    emailVerified: false,
  };
  assert.equal(canAccessProtectedFeatures(unverified), false);

  const suspended: SessionUser = {
    ...verifiedActive,
    accountStatus: 'suspended',
  };
  assert.equal(canAccessProtectedFeatures(suspended), false);

  assert.equal(canAccessProtectedFeatures(null), false);
});

test('canManageAccounts and canInstruct verify authorization levels', () => {
  const student: SessionUser = {
    userId: 'usr_std',
    email: 'student@example.com',
    emailVerified: true,
    roles: ['student'],
    accountStatus: 'active',
  };
  assert.equal(canManageAccounts(student), false);
  assert.equal(canInstruct(student), false);

  const instructor: SessionUser = {
    userId: 'usr_inst',
    email: 'inst@example.com',
    emailVerified: true,
    roles: ['student', 'instructor'],
    accountStatus: 'active',
  };
  assert.equal(canManageAccounts(instructor), false);
  assert.equal(canInstruct(instructor), true);

  const admin: SessionUser = {
    userId: 'usr_adm',
    email: 'admin@example.com',
    emailVerified: true,
    roles: ['student', 'admin'],
    accountStatus: 'active',
  };
  assert.equal(canManageAccounts(admin), true);
  assert.equal(canInstruct(admin), true);
});
