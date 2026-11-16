import {expect, test} from '@playwright/test';

test('generated API contract matches the running service', async ({request}) => {
  const response = await request.get('/api/ai2workspace/v1/health');
  expect(response.status()).toBe(200);
  expect(await response.json()).toEqual({service: 'ai2workspace', status: 'ok'});
});
