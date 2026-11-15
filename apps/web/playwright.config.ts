import { defineConfig } from '@playwright/test';
import { createTestRegistry } from './e2e/fixtures';

const externalURL = process.env.WORKSPACE_TEST_URL;
const port = process.env.WORKSPACE_TEST_PORT ?? '1140';
const baseURL = externalURL ?? `http://127.0.0.1:${port}`;
const registry = externalURL ? undefined : createTestRegistry();

export default defineConfig({
  testDir: 'e2e',
  workers: 1,
  timeout: 45000,
  reporter: 'line',
  use: {
    baseURL,
    viewport: { width: 1440, height: 1000 },
    screenshot: 'only-on-failure',
  },
  webServer: externalURL
    ? undefined
    : {
        command: `pnpm exec next start --hostname 127.0.0.1 --port ${port}`,
        env: { AI2WORKSPACE_SPACES_DIR: registry! },
        url: baseURL,
        reuseExistingServer: false,
        timeout: 60000,
      },
});
