import { defineConfig } from '@playwright/test';

const externalURL = process.env.WORKSPACE_TEST_URL;
const baseURL = externalURL ?? 'http://127.0.0.1:1140';

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
        command: 'pnpm start',
        url: baseURL,
        reuseExistingServer: false,
        timeout: 60000,
      },
});
