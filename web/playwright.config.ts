import { defineConfig, devices } from '@playwright/test'
import process from 'node:process'

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  reporter: 'list',
  use: {
    baseURL: 'http://127.0.0.1:13000',
    trace: 'retain-on-failure',
    launchOptions: process.env.PLAYWRIGHT_CHROME_PATH
      ? { executablePath: process.env.PLAYWRIGHT_CHROME_PATH }
      : {},
  },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile', use: { ...devices['iPhone 13'], defaultBrowserType: 'chromium' } },
  ],
  webServer: [
    {
      command: 'go run ../cmd/api',
      url: 'http://127.0.0.1:18080/health/ready',
      timeout: 120000,
      reuseExistingServer: false,
      env: { DATABASE_URL: process.env.TEST_DATABASE_URL || '', API_LISTEN: '127.0.0.1:18080' },
    },
    {
      command: 'node .output/server/index.mjs',
      url: 'http://127.0.0.1:13000',
      timeout: 60000,
      reuseExistingServer: false,
      env: { NUXT_API_BASE: 'http://127.0.0.1:18080', HOST: '127.0.0.1', PORT: '13000' },
    },
  ],
})
