import {test} from 'node:test';
import assert from 'node:assert/strict';
import {
  validateVideoFile,
  MAX_VIDEO_BYTES,
} from '../src/features/media/video_uploader';

test('validateVideoFile allows valid MP4 files within limits', () => {
  const result = validateVideoFile({
    name: 'lesson_1.mp4',
    size: 1024 * 1024 * 50, // 50 MiB
  });
  assert.equal(result.valid, true);
  assert.equal(result.error, undefined);
});

test('validateVideoFile rejects files exceeding 2 GiB boundary', () => {
  const result = validateVideoFile({
    name: 'oversized.mp4',
    size: MAX_VIDEO_BYTES + 1,
  });
  assert.equal(result.valid, false);
  assert.match(result.error || '', /2 GiB/);
});

test('validateVideoFile rejects empty files', () => {
  const result = validateVideoFile({
    name: 'empty.mp4',
    size: 0,
  });
  assert.equal(result.valid, false);
  assert.match(result.error || '', /大于 0/);
});

test('validateVideoFile rejects non-mp4 containers', () => {
  const testCases = ['video.mov', 'video.avi', 'video.mkv', 'video.wmv'];
  for (const name of testCases) {
    const result = validateVideoFile({name, size: 1000});
    assert.equal(result.valid, false);
    assert.match(result.error || '', /MP4/);
  }
});
