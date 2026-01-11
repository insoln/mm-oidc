---
name: e2e-testing
description: Develop and run end-to-end tests using Playwright for the Mattermost OIDC plugin. Use this skill when writing new tests, debugging test failures, or integrating tests into CI/CD pipelines.
license: Apache-2.0
metadata:
  author: insoln
  version: "1.0"
  category: testing
  tags: [playwright, e2e, testing, automation, ci-cd]
---

# E2E Testing Skill

## Summary
This skill covers end-to-end testing with Playwright including test development, execution, debugging, and CI/CD integration for the Mattermost OIDC plugin.

## When to Use This Skill
- Writing new E2E tests
- Running test suite locally or in CI
- Debugging test failures
- Analyzing test reports
- Improving test coverage

## Test Infrastructure

### Test Structure
```
e2e/
├── package.json          # Dependencies
├── playwright.config.ts  # Configuration
├── tests/               # Test suites
│   ├── health.spec.ts   # Health checks
│   ├── oidc-flow.spec.ts # Auth flow
│   └── proxy-redirect.spec.ts # Proxy tests
└── playwright-report/   # HTML reports
```

### Prerequisites
- Node.js 20+ with Corepack
- Running dev stack (or target environment)
- Chromium browser (installed via Playwright)

## Running Tests

### Install Dependencies
```bash
make e2e-install

# Or manually
cd e2e
yarn install --inline-builds
yarn playwright install chromium
```

### Run All Tests
```bash
# Using script (starts dev stack automatically)
./scripts/e2e-test.sh

# Or using make
make e2e-test

# Or directly with Playwright
cd e2e
yarn test
```

### Run Specific Tests
```bash
cd e2e

# Single test file
yarn test tests/oidc-flow.spec.ts

# Specific test case
yarn test tests/oidc-flow.spec.ts -g "should complete full OIDC login flow"

# Multiple files
yarn test tests/health.spec.ts tests/oidc-flow.spec.ts
```

### Run with Options
```bash
# Headed mode (visible browser)
yarn test:headed

# Debug mode (step through tests)
yarn test:debug

# Update snapshots
yarn test --update-snapshots

# Run against custom URL
MM_SITE_URL=https://chat.example.com yarn test
```

## Writing New Tests

### Basic Test Template
```typescript
import { test, expect } from '@playwright/test';

test.describe('Feature Name', () => {
  test('should do something specific', async ({ page }) => {
    // Navigate
    await page.goto('/target-page');
    
    // Interact
    await page.click('button[data-testid="action"]');
    
    // Assert
    await expect(page.locator('.result')).toBeVisible();
  });
});
```

### OIDC Flow Test Example
```typescript
test('should complete OIDC login', async ({ page }) => {
  // Clear cookies for fresh state
  await page.context().clearCookies();
  
  // Navigate to plugin landing page
  await page.goto('/plugins/com.mm.oidc/');
  
  // Start login
  await page.click('text=Start Login');
  
  // Wait for Keycloak redirect
  await page.waitForURL(/keycloak/);
  
  // Authenticate
  await page.fill('input[name="username"]', process.env.KC_ADMIN || 'admin');
  await page.fill('input[name="password"]', process.env.KC_ADMIN_PASSWORD || 'Keycloak123!');
  await page.click('input[type="submit"]');
  
  // Wait for callback and redirect
  await page.waitForURL(/mattermost/);
  
  // Verify session cookie
  const cookies = await page.context().cookies();
  const authCookie = cookies.find(c => c.name === 'MMAUTHTOKEN');
  expect(authCookie).toBeDefined();
  expect(authCookie?.httpOnly).toBe(true);
  
  // Verify logged in
  await expect(page.locator('.user-popover')).toBeVisible({ timeout: 10000 });
});
```

### Best Practices

**State Management:**
- Clear cookies between tests
- Use `test.beforeEach()` for setup
- Clean up in `test.afterEach()`

**Selectors:**
- Prefer `data-testid` attributes
- Use semantic locators (role, text)
- Avoid brittle CSS selectors

**Assertions:**
- Use explicit waits: `toBeVisible()`, `toHaveText()`
- Set appropriate timeouts for OIDC flows
- Verify multiple conditions

**Error Handling:**
- Add descriptive test names
- Use `test.fail()` for expected failures
- Capture screenshots/videos on failure

## Debugging Tests

### Local Debugging

```bash
# Run in debug mode
cd e2e
yarn test:debug tests/oidc-flow.spec.ts

# Playwright Inspector opens
# - Step through test
# - Inspect page state
# - Try selectors
# - View network requests
```

### Headed Mode
```bash
# See browser during test execution
yarn test:headed

# Slow down test execution
yarn test --headed --slow-mo=1000
```

### Console Logs
```typescript
test('debug example', async ({ page }) => {
  // Log page console messages
  page.on('console', msg => console.log('PAGE LOG:', msg.text()));
  
  // Log network requests
  page.on('request', req => console.log('REQUEST:', req.url()));
  page.on('response', res => console.log('RESPONSE:', res.url(), res.status()));
  
  // Your test code...
});
```

### Screenshots and Videos
```bash
# Screenshots saved to test-results/ on failure
# Videos saved if configured in playwright.config.ts

# View traces
npx playwright show-trace trace.zip
```

## Test Reports

### HTML Report
```bash
# Generate and open report
cd e2e
yarn test:report

# Or manually
npx playwright show-report
```

Report includes:
- Test results and duration
- Screenshots on failure
- Network activity
- Console logs
- Traces (if enabled)

### CI Reports
GitHub Actions automatically uploads:
- HTML test reports as artifacts
- Test result summaries
- Screenshots/videos of failures

## CI/CD Integration

### GitHub Actions
```yaml
# .github/workflows/e2e-tests.yml
name: E2E Tests
on: [push, pull_request]

jobs:
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Node.js
        uses: actions/setup-node@v3
        with:
          node-version: '20'
          
      - name: Start dev stack
        run: ./scripts/dev-up.sh
        
      - name: Run E2E tests
        run: ./scripts/e2e-test.sh
        
      - name: Upload report
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: playwright-report
          path: e2e/playwright-report/
          
      - name: Teardown
        if: always()
        run: ./scripts/dev-down.sh
```

### Environment Variables
```bash
# Test configuration
export MM_SITE_URL=http://localhost:8065
export KC_ADMIN=admin
export KC_ADMIN_PASSWORD=Keycloak123!
export CI=true # Enables CI-specific behavior
export SKIP_DEV_STACK=false # Set true if stack already running
```

## Configuration

### Playwright Config
```typescript
// playwright.config.ts
export default defineConfig({
  testDir: './tests',
  fullyParallel: false, // Avoid session conflicts
  workers: 1, // Sequential execution
  retries: process.env.CI ? 2 : 0,
  reporter: [
    ['html'],
    ['list'],
    ['github'] // Only in CI
  ],
  use: {
    baseURL: process.env.MM_SITE_URL || 'http://localhost:8065',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    trace: 'on-first-retry',
    actionTimeout: 10000,
    navigationTimeout: 30000,
  },
});
```

## Test Coverage

### Current Coverage
- ✅ Plugin health endpoint
- ✅ Landing page rendering
- ✅ Complete OIDC login flow
- ✅ Invalid credentials handling
- ✅ Proxy redirect behavior
- ✅ API exclusion from redirects
- ✅ WebSocket functionality

### Gaps to Address
- ⬜ Logout flow
- ⬜ Admin role promotion
- ⬜ Profile synchronization
- ⬜ Error recovery scenarios
- ⬜ Mobile viewport testing
- ⬜ Accessibility checks

## Troubleshooting

### Tests Timeout
```typescript
// Increase timeouts for slow operations
test('slow operation', async ({ page }) => {
  await page.goto('/plugins/com.mm.oidc/', { 
    timeout: 60000 // 60 seconds
  });
});

// Or globally in playwright.config.ts
use: {
  navigationTimeout: 60000,
}
```

### Dev Stack Not Ready
```bash
# Ensure stack is fully healthy before tests
./scripts/dev-up.sh

# Check services
docker compose -f deploy/docker-compose.dev.yml ps

# Verify plugin health
curl http://localhost:8065/plugins/com.mm.oidc/health
```

### Browser Issues
```bash
# Reinstall browsers
cd e2e
yarn playwright install --force chromium

# Clear cache
rm -rf ~/.cache/ms-playwright
yarn playwright install chromium
```

## Related Skills
- dev-environment
- oidc-flow-test
- troubleshooting

## References
- [docs/E2E_TESTING.md](../../../docs/E2E_TESTING.md)
- [Playwright Documentation](https://playwright.dev)
- [e2e/tests/](../../../e2e/tests/)
