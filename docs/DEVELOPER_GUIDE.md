# Mattermost OIDC Plugin – Developer Guide

This guide documents the workflows that Mattermost OIDC plugin developers and contributors use while building from source, validating changes locally, and exercising the automated regression suites. Operators looking for installation steps should continue to use [docs/USER_GUIDE.md](USER_GUIDE.md).

---

## 1. Build and package from source

1. **Install prerequisites**
   - Go 1.22+
   - Node.js 20.x via Corepack/Yarn 4 (run `corepack enable` if needed)
   - Docker 25+ for the local stack
2. **Install webapp dependencies**
   ```bash
   make webapp-install
   ```
3. **Compile the plugin bundle**
   ```bash
   make package
   ```
   The signed archive is written to `build/plugins/mm-oidc.tar.gz` and mirrors the artifact published on GitHub Releases. Upload this file to a Mattermost server or hand it to CI/CD systems.
4. **Incremental development**
   - Run `make server-build` / `make webapp-build` when you need targeted builds.
   - Use `make server-test` and `make webapp-test` to execute Go and React unit tests respectively.

---

## 2. Use the bundled Docker dev stack

The repository ships with a Docker Compose environment (Mattermost + Keycloak + Postgres + proxy) for local development and QA.

**Quick reference:**

```bash
# Start the stack (builds plugin, provisions Keycloak, configures Mattermost)
./scripts/dev-up.sh

# View logs
./scripts/dev-logs.sh mattermost keycloak

# Test the login flow
# Open http://mattermost-proxy.127.0.0.1.nip.io:8787
# Navigate to /plugins/com.mm.oidc/ and click "Start Login"

# Stop the stack
./scripts/dev-down.sh
```

For complete setup details, prerequisites, environment customization, Keycloak bootstrap automation, and troubleshooting, see **[docs/DEV_ENV.md](DEV_ENV.md)**.

---

## 3. Automated regression suites

### 3.1 Playwright end-to-end tests

```bash
./scripts/e2e-test.sh
```

- Builds the plugin if necessary, boots the dev stack, and executes `e2e/tests/oidc-flow.spec.ts`.
- Mirrors the user-facing installation and login flow documented in [docs/USER_GUIDE.md](USER_GUIDE.md).

### 3.2 Proxy validation harness

```bash
./scripts/test-proxy.sh       # curl smoke tests
./scripts/test-proxy-all.sh   # curl + Playwright (tests/proxy-redirect.spec.ts)
```

- Confirms unauthenticated requests are redirected, API/JSON calls bypass the proxy rules, and WebSockets remain functional.
- Designed for CI pipelines as well as local troubleshooting when editing [deploy/mattermost-proxy/nginx.conf](../deploy/mattermost-proxy/nginx.conf). Details about the redirect strategy live in [docs/PROXY_GUIDE.md](PROXY_GUIDE.md).

---

## 4. Additional references

- Architecture and security deep dive – [docs/ARCHITECTURE.md](ARCHITECTURE.md)
- Development environment knobs – [docs/DEV_ENV.md](DEV_ENV.md)
- End-to-end testing notes – [docs/E2E_TESTING.md](E2E_TESTING.md)
- Proxy behavior and rationale – [docs/PROXY_GUIDE.md](PROXY_GUIDE.md)

Keep this guide updated whenever scripts or workflows change so contributors have a single source of truth for local development.
