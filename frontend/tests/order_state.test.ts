import {test} from 'node:test';
import assert from 'node:assert/strict';
import {formatPrice, isRefundEligible} from '../src/features/commerce/checkout';

test('formatPrice correctly formats minor currency amounts to yuan', () => {
  assert.equal(formatPrice(9900, 'CNY'), '¥99.00');
  assert.equal(formatPrice(50, 'CNY'), '¥0.50');
  assert.equal(formatPrice(0, 'CNY'), '¥0.00');
  assert.equal(formatPrice(199900, 'USD'), 'USD 1999.00');
});

test('isRefundEligible returns true within 7x24h window for paid orders', () => {
  const now = new Date('2026-11-20T12:00:00Z');
  const paidAt = new Date('2026-11-15T12:00:00Z').toISOString(); // 5 days ago

  const eligible = isRefundEligible('paid', paidAt, now);
  assert.equal(eligible, true);
});

test('isRefundEligible returns false beyond 7x24h window', () => {
  const now = new Date('2026-11-25T12:00:00Z');
  const paidAt = new Date('2026-11-15T12:00:00Z').toISOString(); // 10 days ago

  const eligible = isRefundEligible('paid', paidAt, now);
  assert.equal(eligible, false);
});

test('isRefundEligible returns false for unpaid or already refunded orders', () => {
  const now = new Date();
  const paidAt = new Date().toISOString();

  assert.equal(isRefundEligible('pending', paidAt, now), false);
  assert.equal(isRefundEligible('refunded', paidAt, now), false);
  assert.equal(isRefundEligible('paid', null, now), false);
});
