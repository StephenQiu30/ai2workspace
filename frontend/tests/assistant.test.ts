import assert from 'node:assert/strict';
import {test} from 'node:test';
import {
  classifyAssistantError,
  computeUsagePercentage,
  extractBudgetPausedJob,
  formatCentsToYuan,
  getAgentStateLabel,
  getProviderLabel,
  isModelExpired,
} from '../src/features/assistant/assistant_helpers';

test('formatCentsToYuan and computeUsagePercentage accurately format AI budget figures', () => {
  assert.equal(formatCentsToYuan(200), '¥2.00');
  assert.equal(formatCentsToYuan(10000), '¥100.00');
  assert.equal(formatCentsToYuan(undefined), '—');

  assert.equal(computeUsagePercentage(80, 100), 80);
  assert.equal(computeUsagePercentage(150, 100), 100);
  assert.equal(computeUsagePercentage(-10, 100), 0);
  assert.equal(computeUsagePercentage(10, 0), 0);
});

test('getProviderLabel and getAgentStateLabel map four providers and agent states', () => {
  assert.equal(getProviderLabel('deepseek'), 'DeepSeek');
  assert.equal(getProviderLabel('openrouter'), 'OpenRouter');
  assert.equal(getProviderLabel('volcengine'), '火山引擎');
  assert.equal(getProviderLabel('openai'), 'OpenAI');

  assert.equal(getAgentStateLabel('draft'), '草稿');
  assert.equal(getAgentStateLabel('published'), '已发布');
  assert.equal(getAgentStateLabel('disabled'), '已停用');
});

test('isModelExpired detects expired price table entries', () => {
  assert.equal(isModelExpired(undefined), false);
  assert.equal(isModelExpired('2020-01-01T00:00:00Z'), true);
  assert.equal(isModelExpired('2099-01-01T00:00:00Z'), false);
});

test('classifyAssistantError distinguishes auth, verification, quota, search and revoked states', () => {
  const unauth = classifyAssistantError({
    response: {status: 401, data: {message: 'unauthorized'}},
  });
  assert.equal(unauth.kind, 'auth');
  assert.equal(unauth.retryable, false);

  const unverified = classifyAssistantError({
    response: {status: 403, data: {message: 'email not verified'}},
  });
  assert.equal(unverified.kind, 'verify');

  const budget = classifyAssistantError({
    response: {status: 402, data: {message: 'daily budget exceeded'}},
  });
  assert.equal(budget.kind, 'quota');

  const search = classifyAssistantError({
    response: {status: 502, data: {message: 'search timeout'}},
  });
  assert.equal(search.kind, 'search');
  assert.equal(search.retryable, true);

  const pausedJob = extractBudgetPausedJob({
    response: {
      status: 402,
      data: {id: 'job_1', state: 'paused_budget', stage: 'budget_admission'},
    },
  });
  assert.equal(pausedJob?.state, 'paused_budget');
});
