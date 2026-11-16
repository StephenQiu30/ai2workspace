import assert from 'node:assert/strict';
import {after, before, test} from 'node:test';
import {createServer} from 'node:http';
import type {AddressInfo} from 'node:net';
import axios from 'axios';
import {request} from '../src/lib/request/transport';

const server = createServer((req, res) => {
  res.setHeader('Content-Type', 'application/json');
  if (req.url === '/fail') {
    res.writeHead(503).end('{"error":"unavailable"}');
    return;
  }
  res.end(
    JSON.stringify({url: req.url, identity: req.headers['x-identity'] ?? null}),
  );
});
let baseURL: string;

before(async () => {
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  baseURL = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
});

after(async () => {
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve())),
  );
});

test('returns the response body and preserves query parameters', async () => {
  const result = await request<{url: string}>('/status', {
    baseURL,
    params: {q: 'ai2workspace'},
  });
  assert.equal(result.url, '/status?q=ai2workspace');
});

test('preserves HTTP failures for the caller', async () => {
  await assert.rejects(
    request('/fail', {baseURL}),
    (error: unknown) =>
      axios.isAxiosError(error) && error.response?.status === 503,
  );
});

test('keeps headers isolated between concurrent calls', async () => {
  const [first, second, anonymous] = await Promise.all([
    request<{identity: string}>('/', {
      baseURL,
      headers: {'X-Identity': 'first'},
    }),
    request<{identity: string}>('/', {
      baseURL,
      headers: {'X-Identity': 'second'},
    }),
    request<{identity: null}>('/', {baseURL}),
  ]);
  assert.equal(first.identity, 'first');
  assert.equal(second.identity, 'second');
  assert.equal(anonymous.identity, null);
});

test('honors request cancellation', async () => {
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(
    request('/', {baseURL, signal: controller.signal}),
    (error: unknown) => axios.isCancel(error),
  );
});
