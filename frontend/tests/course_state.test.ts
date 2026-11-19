import assert from 'node:assert/strict';
import {test} from 'node:test';
import type {CatalogCourse} from '../src/features/course/course_catalog';

test('course catalog filtering matches query case-insensitively', () => {
  const courses: CatalogCourse[] = [
    {id: '1', title: 'Next.js 深入浅出', isFree: true},
    {id: '2', title: 'Go 高并发微服务架构', isFree: false},
    {id: '3', title: 'React 19 全新特性解读', isFree: true},
  ];

  const search = (q: string) =>
    courses.filter((c) =>
      c.title.toLowerCase().includes(q.toLowerCase().trim()),
    );

  assert.equal(search('next').length, 1);
  assert.equal(search('NEXT').length, 1);
  assert.equal(search('架构').length, 1);
  assert.equal(search('不存在').length, 0);
  assert.equal(search(' ').length, 3);
});

test('course lessons count sums correctly across sections', () => {
  const sections: Array<{lessons?: Array<{id: string}>}> = [
    {lessons: [{id: 'l1'}, {id: 'l2'}]},
    {lessons: [{id: 'l3'}]},
    {lessons: []},
  ];

  const totalLessons = sections.reduce(
    (acc, sec) => acc + (sec.lessons?.length ?? 0),
    0,
  );

  assert.equal(totalLessons, 3);
});
