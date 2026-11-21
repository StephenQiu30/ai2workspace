import assert from 'node:assert/strict';
import test from 'node:test';
import {
  SERVICE_CATALOG,
  checkAllServicesHealth,
  summarizeServiceHealth,
  type HealthFetcher,
  type ServiceId,
} from '../src/features/service_status/service_registry';

test('SERVICE_CATALOG registers all 5 microservices with expected ports', () => {
  assert.deepEqual(
    SERVICE_CATALOG.map((service) => [service.id, service.port]),
    [
      ['identity', 3123],
      ['learning', 3124],
      ['commerce', 3125],
      ['community', 3126],
      ['assistant', 3127],
    ],
  );
});

test('checkAllServicesHealth records healthy and unavailable services independently', async () => {
  const fetchers: Record<ServiceId, HealthFetcher> = {
    identity: async () => ({service: 'identity', status: 'ok'}),
    learning: async () => ({service: 'learning', status: 'ok'}),
    commerce: async () => {
      throw new Error('connection refused');
    },
    community: async () => ({service: 'community', status: 'ok'}),
    assistant: async () => ({service: 'assistant', status: 'degraded'}),
  };

  const records = await checkAllServicesHealth(undefined, fetchers);
  const summary = summarizeServiceHealth(records);

  assert.equal(records.length, 5);
  assert.equal(summary.total, 5);
  assert.equal(summary.healthy, 3);
  assert.equal(summary.unhealthy, 2);
  assert.equal(summary.allHealthy, false);

  const commerce = records.find((item) => item.id === 'commerce');
  assert.equal(commerce?.available, false);
  assert.equal(commerce?.status, 'unavailable');

  const assistant = records.find((item) => item.id === 'assistant');
  assert.equal(assistant?.available, false);
  assert.equal(assistant?.status, 'degraded');
});
