# Playwright E2E Tests

End-to-end tests for the mm-oidc plugin using Playwright. Tests validate the complete OIDC authentication flow against a running Mattermost + Keycloak stack.

## Quick Start

```bash
# From project root
make e2e-install  # Install dependencies
make e2e-test     # Run tests

# Or use script directly
./scripts/e2e-test.sh
```

## Full Documentation

See **[docs/E2E_TESTING.md](../docs/E2E_TESTING.md)** for complete details on:
- Test structure and test suites
- Running tests locally (headed, debug, reports)
- CI integration and workflow
- Configuration and environment variables
- Test development best practices
- Troubleshooting and debugging
- Future enhancements
