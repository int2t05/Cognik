import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './test',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [
    ['html', { open: 'never' }],
    ['list'],
  ],
  use: {
    baseURL: 'http://127.0.0.1:3000',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    // API 集成测试（仅需要 Go 后端，串行执行）
    {
      name: 'api',
      use: { ...devices['Desktop Chrome'] },
      testMatch: /test\/api\/.*\.spec\.ts$/,
      fullyParallel: false,
    },
    // E2E 浏览器测试（需要 Next.js 前端 + Go 后端）
    {
      name: 'e2e',
      use: { ...devices['Desktop Chrome'] },
      testMatch: /test\/(?!api\/).*\.spec\.ts$/,
      fullyParallel: true,
    },
  ],
});
