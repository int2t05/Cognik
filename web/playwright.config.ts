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
    // E2E 浏览器测试（需要 Next.js 前端 + Go 后端）
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
      testMatch: /^(?!.*\/api\/).*\.spec\.ts$/,
    },
    // API 集成测试（仅需要 Go 后端）
    {
      name: 'api',
      use: { ...devices['Desktop Chrome'] },
      testMatch: /test\/api\/.*\.spec\.ts$/,
      // API 测试串行执行（共享数据库状态）
      fullyParallel: false,
    },
  ],
});
