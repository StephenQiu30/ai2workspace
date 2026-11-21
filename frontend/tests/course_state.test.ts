import assert from 'node:assert/strict';
import {test} from 'node:test';
import {
  countCourseLessons,
  flattenCourseLessons,
  formatDurationSeconds,
  getCourseCombinedStatusMeta,
  sortSectionsAndLessons,
} from '../src/features/course/course_status';

test('getCourseCombinedStatusMeta prioritizes withdrawn and sales_closed over productionState', () => {
  assert.equal(
    getCourseCombinedStatusMeta({
      productionState: 'approved',
      saleState: 'on_sale',
      deliveryState: 'withdrawn',
    }).label,
    '已撤回',
  );

  assert.equal(
    getCourseCombinedStatusMeta({
      productionState: 'approved',
      saleState: 'sales_closed',
      deliveryState: 'deliverable',
    }).label,
    '已停售',
  );

  assert.equal(
    getCourseCombinedStatusMeta({
      productionState: 'approved',
      saleState: 'on_sale',
      deliveryState: 'deliverable',
    }).label,
    '已发布',
  );
});

test('sortSectionsAndLessons and flattenCourseLessons order curriculum deterministically', () => {
  const sections = [
    {
      id: 'sec_2',
      title: '第二章',
      sortOrder: 2,
      lessons: [
        {id: 'les_2b', title: '2.2', sortOrder: 2},
        {id: 'les_2a', title: '2.1', sortOrder: 1},
      ],
    },
    {
      id: 'sec_1',
      title: '第一章',
      sortOrder: 1,
      lessons: [{id: 'les_1a', title: '1.1', sortOrder: 1}],
    },
  ];

  const sorted = sortSectionsAndLessons(sections);
  assert.equal(sorted[0]?.id, 'sec_1');
  assert.equal(sorted[1]?.lessons?.[0]?.id, 'les_2a');

  const flat = flattenCourseLessons(sections);
  assert.deepEqual(
    flat.map((item) => item.lesson.id),
    ['les_1a', 'les_2a', 'les_2b'],
  );
  assert.equal(countCourseLessons({sections}), 3);
});

test('formatDurationSeconds formats hours, minutes and seconds clearly', () => {
  assert.equal(formatDurationSeconds(0), '0 秒');
  assert.equal(formatDurationSeconds(45), '45 秒');
  assert.equal(formatDurationSeconds(125), '2 分 5 秒');
  assert.equal(formatDurationSeconds(3720), '1 小时 2 分');
});
