import { defineConfig, devices } from '@playwright/test';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const currentDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(currentDir, '..');

/**
 * Playwright configuration for mm-oidc E2E tests
 * Tests run against the local dev stack (Mattermost + Keycloak)
 */
export default defineConfig({
  testDir: './tests',
  fullyParallel: false, // OIDC flows need sequential execution to avoid state conflicts
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : 1, // Single worker to avoid Keycloak session conflicts
  reporter: process.env.CI ? [['html'], ['github']] : [['html'], ['list']],
  
  use: {
    baseURL: process.env.MM_SITE_URL || 'http://localhost:8065',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],

  // Run dev stack before tests if not already running
  webServer: process.env.SKIP_DEV_STACK ? undefined : {
    command: `bash -c "cd '${repoRoot}' && ./scripts/dev-up.sh"`,
    url: 'http://localhost:8065',
    timeout: 180 * 1000, // 3 minutes for stack to come up
    reuseExistingServer: !process.env.CI,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
