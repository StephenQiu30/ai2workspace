import assert from 'node:assert/strict';
import {test} from 'node:test';
import {
  isSafeUrl,
  sanitizeRichTextHtml,
} from '../src/components/editor/tiptap_renderer';
import {
  buildLoginHref,
  getTopicMeta,
  sanitizeNextPath,
} from '../src/lib/format';

test('sanitizeNextPath and buildLoginHref strictly block open redirects (R-24)', () => {
  assert.equal(sanitizeNextPath('/community/post_1'), '/community/post_1');
  assert.equal(sanitizeNextPath('https://evil.example.com'), '/');
  assert.equal(sanitizeNextPath('//evil.example.com'), '/');
  assert.equal(sanitizeNextPath('/account?mode=login'), '/');

  assert.equal(
    buildLoginHref('/community/post_1'),
    '/account?next=%2Fcommunity%2Fpost_1',
  );
  assert.equal(buildLoginHref('/'), '/account');
});

test('getTopicMeta resolves standard community topics and custom slugs', () => {
  const method = getTopicMeta('learning-method');
  assert.equal(method.label, '学习方法');

  const fallback = getTopicMeta('custom-topic');
  assert.equal(fallback.slug, 'custom-topic');
});

test('sanitizeRichTextHtml strips scripts, unsafe attributes and javascript: URLs (R-26)', () => {
  const dirty =
    '<h1>主标题降级</h1><p onclick="alert(1)">安全正文<script>alert("xss")</script></p><a href="javascript:alert(1)">危险链接</a><a href="https://learnis.example.com">合法外链</a>';
  const clean = sanitizeRichTextHtml(dirty);

  assert.ok(clean.includes('<h2>主标题降级</h2>'));
  assert.ok(!clean.includes('<script'));
  assert.ok(!clean.includes('onclick'));
  assert.ok(!clean.includes('javascript:'));
  assert.ok(
    clean.includes(
      '<a href="https://learnis.example.com" target="_blank" rel="noopener noreferrer">合法外链</a>',
    ),
  );
});

test('isSafeUrl allows http, https and internal paths while rejecting script protocols', () => {
  assert.equal(isSafeUrl('https://example.com/image.png'), true);
  assert.equal(isSafeUrl('/courses/c_1'), true);
  assert.equal(isSafeUrl('//evil.com'), false);
  assert.equal(isSafeUrl('javascript:void(0)'), false);
  assert.equal(isSafeUrl('data:text/html,<script>alert(1)</script>'), false);
});
