import {expect, test} from '@playwright/test';

test('server rendering and browser refresh use the real backend', async ({page}) => {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  await page.goto('/');
  await expect(page.getByRole('heading', {name: 'ai2workspace', exact: true})).toBeVisible();
  await expect(page.getByText('连接正常', {exact: true})).toBeVisible();
  const request = page.waitForResponse('**/api/ai2workspace/v1/health');
  const refresh = page.getByRole('button', {name: '重新检查'});
  await refresh.focus();
  await expect(refresh).toBeFocused();
  await refresh.press('Enter');
  expect((await request).status()).toBe(200);
  await expect(page.getByText('连接正常', {exact: true})).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect(errors).toEqual([]);
});

test('an interrupted browser request can recover', async ({page}) => {
  await page.goto('/');
  await page.route('**/api/ai2workspace/v1/health', (route) => route.abort('connectionfailed'));
  await page.getByRole('button', {name: '重新检查'}).click();
  await expect(page.getByRole('alert').filter({hasText: '服务暂不可用'})).toBeVisible();
  await page.unroute('**/api/ai2workspace/v1/health');
  await page.getByRole('button', {name: '重新检查'}).click();
  await expect(page.getByText('连接正常', {exact: true})).toBeVisible();
});
