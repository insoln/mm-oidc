# Playwright E2E Tests

This directory contains end-to-end tests for the mm-oidc plugin using Playwright.

## Quick Start

```bash
# From project root
make e2e-install  # Install dependencies
make e2e-test     # Run tests

# Or use script directly
./scripts/e2e-test.sh
```

## What's Tested

- Plugin health endpoint
- Plugin landing page
- Complete OIDC authentication flow (Authorization Code + PKCE)
- Keycloak integration
- Error handling

## Documentation

See **[docs/E2E_TESTING.md](../docs/E2E_TESTING.md)** for:
- Detailed test documentation
- Local development guide
- CI integration details
- Troubleshooting tips
- Configuration options

## Running Tests

```bash
# All tests
yarn test

# Headed mode (visible browser)
yarn test:headed

# Debug mode
yarn test:debug

# View report
yarn test:report
```

## Configuration

- `playwright.config.ts` - Playwright configuration
- Environment variables in `deploy/env/dev.env`
- Automatically starts dev stack unless `SKIP_DEV_STACK=true`
