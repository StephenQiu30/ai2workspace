import {test} from 'node:test';
import assert from 'node:assert/strict';

export function computeNextReaction(
  current: 'like' | 'dislike' | 'none',
  target: 'like' | 'dislike',
): 'like' | 'dislike' | 'none' {
  if (current === target) {
    return 'none';
  }
  return target;
}

export function truncateSummary(text: string, maxLength = 100): string {
  const runes = Array.from(text);
  if (runes.length <= maxLength) {
    return text;
  }
  return runes.slice(0, maxLength).join('') + '...';
}

test('computeNextReaction enforces mutual exclusion between like and dislike', () => {
  // 1. From none to like
  assert.equal(computeNextReaction('none', 'like'), 'like');

  // 2. From like to like (toggle cancel)
  assert.equal(computeNextReaction('like', 'like'), 'none');

  // 3. From like to dislike (switch)
  assert.equal(computeNextReaction('like', 'dislike'), 'dislike');

  // 4. From dislike to dislike (toggle cancel)
  assert.equal(computeNextReaction('dislike', 'dislike'), 'none');

  // 5. From dislike to like (switch)
  assert.equal(computeNextReaction('dislike', 'like'), 'like');
});

test('truncateSummary accurately limits preview length and preserves short strings', () => {
  const shortText = '短文本无需截断';
  assert.equal(truncateSummary(shortText, 20), shortText);

  const longText = 'A'.repeat(120);
  const truncated = truncateSummary(longText, 100);
  assert.equal(truncated.length, 103); // 100 characters + '...'
  assert.ok(truncated.endsWith('...'));
});
