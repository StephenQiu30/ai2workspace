import {test} from 'node:test';
import assert from 'node:assert/strict';

export function calculateRemainingQuestions(maxDaily = 20, used = 0): number {
  return Math.max(0, maxDaily - used);
}

export function formatSearchEvidenceSummary(
  queries: Array<{title: string}> = [],
): string {
  if (queries.length === 0) return '无外部网络证据';
  return queries.map((q) => q.title).join('；');
}

export function formatCitationsSummary(
  citations: Array<{title: string}> = [],
): string {
  if (citations.length === 0) return '无知识库引用';
  return citations.map((c) => c.title).join('、');
}

test('calculateRemainingQuestions accurately enforces non-negative bounds', () => {
  assert.equal(calculateRemainingQuestions(20, 0), 20);
  assert.equal(calculateRemainingQuestions(20, 15), 5);
  assert.equal(calculateRemainingQuestions(20, 20), 0);
  assert.equal(calculateRemainingQuestions(20, 25), 0);
});

test('formatSearchEvidenceSummary merges titles separated by semicolon', () => {
  assert.equal(formatSearchEvidenceSummary([]), '无外部网络证据');
  assert.equal(
    formatSearchEvidenceSummary([
      {title: 'Go 并发模型'},
      {title: 'Gin 路由实践'},
    ]),
    'Go 并发模型；Gin 路由实践',
  );
});

test('formatCitationsSummary formats citation titles separated by comma', () => {
  assert.equal(formatCitationsSummary([]), '无知识库引用');
  assert.equal(
    formatCitationsSummary([{title: '第1章 基础'}, {title: '第2章 架构'}]),
    '第1章 基础、第2章 架构',
  );
});
