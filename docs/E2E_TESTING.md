# E2E Testing with Playwright

This document describes the end-to-end (E2E) testing infrastructure for the mm-oidc plugin using Playwright.

## Overview

The E2E test suite validates the complete OIDC authentication flow and plugin functionality in a real browser environment against a running Mattermost + Keycloak stack. Tests are automatically executed in CI and can be run locally for development and debugging.

## Test Structure

```
e2e/
├── package.json           # Playwright dependencies and scripts
├── playwright.config.ts   # Playwright configuration
├── tests/                 # Test suites
│   ├── health.spec.ts    # Plugin health and landing page tests
│   └── oidc-flow.spec.ts # Full OIDC authentication flow tests
├── .gitignore
└── playwright-report/    # Generated HTML test reports (gitignored)
```

## Test Suites

### Health Tests (`health.spec.ts`)
- **Plugin Health Check**: Validates `/plugins/com.mm.oidc/health` endpoint returns healthy status
- **Landing Page**: Verifies the plugin landing page loads and displays expected content

### OIDC Flow Tests (`oidc-flow.spec.ts`)
- **Complete Login Flow**: End-to-end test of Authorization Code + PKCE flow
  1. Navigate to plugin landing page
  2. Click "Start Login" button
  3. Redirect to Keycloak
  4. Authenticate with Keycloak credentials
  5. Redirect back to Mattermost with auth code
  6. Verify successful login and Mattermost session
- **Invalid Credentials**: Tests error handling for wrong username/password
- **State Preservation**: Validates OIDC parameters (state, nonce, PKCE challenge)

## Running Tests Locally

### Prerequisites
- Node.js 20+ with Corepack enabled
- Docker and Docker Compose (for dev stack)
- Running dev stack (Mattermost + Keycloak)

### Quick Start

```bash
# Install dependencies and browsers
make e2e-install

# Run all E2E tests (automatically starts dev stack if needed)
make e2e-test

# Or use the script directly
./scripts/e2e-test.sh
```

### Advanced Usage

```bash
cd e2e

# Run tests in headed mode (visible browser)
yarn test:headed

# Run tests in debug mode (step through tests)
yarn test:debug

# View last test report
yarn test:report

# Run specific test file
yarn test tests/health.spec.ts

# Run tests with custom environment
MM_SITE_URL=http://localhost:8065 yarn test
```

### Skip Dev Stack Startup

If the dev stack is already running:

```bash
SKIP_DEV_STACK=true ./scripts/e2e-test.sh
```

Or set `SKIP_DEV_STACK=true` in your environment to disable automatic stack startup in `playwright.config.ts`.

## CI Integration

E2E tests run automatically on every push and pull request via GitHub Actions (`.github/workflows/ci.yml`).

### CI Workflow

1. **Parallel Setup**: Server and webapp unit tests run first
2. **Dev Stack Bootstrap**: Mattermost + Keycloak containers start
3. **E2E Tests**: Playwright tests execute against the dev stack
4. **Artifact Upload**: Test reports and results uploaded for review
5. **Cleanup**: Dev stack tears down (always runs, even on failure)

### CI Job Details

```yaml
e2e-tests:
  name: Playwright E2E Tests
  runs-on: ubuntu-latest
  needs:
    - server-tests
    - webapp-tests
  steps:
    # ... setup steps ...
    - Run Playwright tests
    - Upload reports (HTML report, test results)
    - Teardown dev stack
```

### Viewing CI Test Reports

1. Navigate to GitHub Actions workflow run
2. Scroll to **Artifacts** section
3. Download `playwright-report` (HTML report with screenshots/videos)
4. Download `playwright-results` (raw test results)
5. Extract and open `index.html` to view the report

## Configuration

### Environment Variables

Tests use these environment variables (with defaults):

- `MM_SITE_URL`: Mattermost URL (default: `http://localhost:8065`)
- `KC_ADMIN`: Keycloak admin username (default: `admin`)
- `KC_ADMIN_PASSWORD`: Keycloak admin password (default: `Keycloak123!`)
- `CI`: Set to `true` in CI environments for special behavior
- `SKIP_DEV_STACK`: Skip automatic dev stack startup (default: `false`)

### Playwright Configuration

`playwright.config.ts` controls test behavior:

- **Parallelization**: Disabled (`workers: 1`) to avoid Keycloak session conflicts
- **Retries**: 2 retries in CI, none locally
- **Reporters**: HTML + GitHub Actions in CI, HTML + list locally
- **Screenshots/Videos**: Captured on failure for debugging
- **Trace**: Captured on first retry

## Test Development

### Writing New Tests

1. Create a new `.spec.ts` file in `e2e/tests/`
2. Import Playwright test utilities:
   ```typescript
   import { test, expect } from '@playwright/test';
   ```
3. Write test suites with descriptive names:
   ```typescript
   test.describe('Feature Name', () => {
     test('should do something specific', async ({ page }) => {
       // Test implementation
     });
   });
   ```

### Best Practices

- **Use descriptive test names**: `should complete full OIDC login flow` not `test login`
- **Clear cookies between tests**: Ensure fresh state with `page.context().clearCookies()`
- **Wait for navigation**: Use `page.waitForURL()` for redirects
- **Verify state**: Check cookies, URLs, and page content after actions
- **Handle timeouts**: OIDC flows can be slow; use appropriate timeouts
- **Sequential execution**: Keep `fullyParallel: false` to avoid session conflicts

### Debugging Failing Tests

```bash
# Run in headed mode to see browser
yarn test:headed

# Run in debug mode with Playwright Inspector
yarn test:debug

# Run specific test
yarn test tests/oidc-flow.spec.ts -g "should complete full OIDC login flow"

# Increase verbosity
DEBUG=pw:api yarn test
```

### Screenshots and Videos

- Screenshots: Captured automatically on test failure
- Videos: Recorded and retained only on failure
- Traces: Captured on first retry, viewable with `npx playwright show-trace`

Location: `e2e/test-results/` (gitignored)

## Troubleshooting

### Tests Fail Locally

1. **Check dev stack status**:
   ```bash
   ./scripts/dev-logs.sh mattermost keycloak
   ```

2. **Verify containers are running**:
   ```bash
   docker compose -f deploy/docker-compose.dev.yml ps
   ```

3. **Restart dev stack**:
   ```bash
   ./scripts/dev-down.sh
   ./scripts/dev-up.sh
   ```

4. **Clear Playwright cache**:
   ```bash
   cd e2e
   rm -rf node_modules .playwright
   yarn install --inline-builds
   yarn playwright install chromium
   ```

### Tests Timeout

- Increase timeout in `playwright.config.ts`:
  ```typescript
  use: {
    actionTimeout: 30000, // 30 seconds
    navigationTimeout: 45000, // 45 seconds
  }
  ```

### Keycloak Authentication Fails

- Check Keycloak credentials in `deploy/env/dev.env`
- Verify Keycloak is healthy: `http://localhost:8080`
- Check test uses correct credentials (env vars)

### CI Tests Pass but Local Tests Fail

- Ensure Docker Compose is using correct env file
- Verify ports 8065 and 8080 are not in use by other processes
- Check `KC_HOSTNAME` matches expected value in `dev.env`

## Future Enhancements

- **Additional test coverage**: Logout flow, role mapping, error scenarios
- **Visual regression testing**: Screenshot comparison for UI changes
- **Performance testing**: Measure OIDC flow latency
- **Multi-browser testing**: Firefox, Safari (currently Chromium-only)
- **Accessibility testing**: a11y checks with Playwright
- **API testing**: Direct API calls to supplement browser tests

## References

- [Playwright Documentation](https://playwright.dev)
- [Playwright Best Practices](https://playwright.dev/docs/best-practices)
- [Mattermost Developer Docs](https://developers.mattermost.com)
- [Keycloak Documentation](https://www.keycloak.org/documentation)
- [OIDC Specification](https://openid.net/specs/openid-connect-core-1_0.html)
