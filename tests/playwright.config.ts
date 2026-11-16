import {defineConfig, devices} from '@playwright/test';

export default defineConfig({
  testDir: '.',
  testMatch: ['contracts/**/*.spec.ts', 'e2e/**/*.spec.ts'],
  fullyParallel: true,
  reporter: 'list',
  use: {baseURL: process.env.BASE_URL ?? 'http://127.0.0.1:1151', channel: 'chrome'},
  projects: [
    {name: 'desktop', use: {viewport: {width: 1440, height: 1000}}},
    {name: 'mobile', use: {...devices['iPhone 13'], defaultBrowserType: 'chromium'}},
  ],
});
