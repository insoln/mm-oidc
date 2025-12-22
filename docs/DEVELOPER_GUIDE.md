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

The repository ships with an opinionated Docker Compose environment (Mattermost, Keycloak, Postgres, proxy) that is ideal for demos, QA, and local development.

### 2.1 Start the stack

```bash
./scripts/dev-up.sh
```

The script performs the following:

1. Copies `deploy/env/dev.env.example` to `deploy/env/dev.env` on first run and exports every variable.
2. Runs `make package` to build `build/plugins/mm-oidc.tar.gz`.
3. Launches [deploy/docker-compose.dev.yml](deploy/docker-compose.dev.yml) and waits for healthy containers.
4. Executes [scripts/dev-bootstrap.sh](scripts/dev-bootstrap.sh) to:
   - Provision the Keycloak realm/client, required mappers, and the `system_admin` client role.
   - Create the Mattermost user `mm-admin` and grant the System Admin permission when the Keycloak role is present.
   - Upload and enable the freshly built plugin, then sync all plugin settings.

### 2.2 Exercise the login flow

1. Open `http://mattermost-proxy.127.0.0.1.nip.io:8787/` in a browser.
2. Sign out if you already have an active Mattermost session.
3. Navigate to `/plugins/com.mm.oidc/` or use the **Start Login** button in the plugin panel.
4. Authenticate with the seeded Keycloak admin credentials (`admin / Keycloak123!`).
5. After the redirect, confirm the Mattermost UI shows `mm-admin` and that the `MMAUTHTOKEN` cookie exists in your browser.

### 2.3 Shut everything down

```bash
./scripts/dev-down.sh
```

Volumes persist so you can resume later. Run `docker compose -f deploy/docker-compose.dev.yml down -v` for a full reset.

### 2.4 Dev-stack limitations

- HTTP endpoints on `127.0.0.1.nip.io` are for local use only—do not expose them to the internet.
- SMTP/SMS MFA integrations are stubbed; only username/password auth is wired in Keycloak.
- The proxy redirects `/login` only for browser GET requests; API clients still hit Mattermost directly on port `8065`.

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
