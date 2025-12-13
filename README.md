# Mattermost OIDC Plugin

Mattermost plugin that enables single sign-on with arbitrary OpenID Connect providers (Keycloak by default). The project tracks the latest Mattermost Server (v9.x+) and Keycloak (v25.x+) releases and follows strict security and coding best practices.

## Installation & Configuration

The plugin ships as a standard Mattermost plugin archive (`mm-oidc.tar.gz`). You can download a prebuilt (https://github.com/insoln/mm-oidc/releases)[release] from GitHub or build it yourself with `make package` (see the Packaging section below).

### 1. Install the plugin in Mattermost

1. Sign in to Mattermost as a system administrator.
2. Navigate to **System Console → Plugin Management → Plugin Upload** and enable plugin uploads if prompted.
3. Upload `mm-oidc.tar.gz` (from `build/plugins/` or the GitHub release) and click **Enable** for `com.mm.oidc`.
4. Open `https://<mattermost-host>/plugins/com.mm.oidc/` to verify the landing page renders and shows your configured issuer/redirect metadata.

### 2. Configure Keycloak (or another OIDC provider)

The bootstrap scripts automate this, but the manual steps mirror what they do:

1. Create (or reuse) a Keycloak realm dedicated to Mattermost.
2. Add a confidential client (e.g., `mattermost`) with **Standard Flow** enabled, **Implicit** and **Direct Access Grants** disabled, and **Service Accounts** disabled.
3. Set **Valid Redirect URIs** to `https://<mattermost-host>/plugins/com.mm.oidc/callback` and **Web Origins** to your Mattermost site URL.
4. Add protocol mappers for `preferred_username`, `given_name`, `family_name`, `full_name`, `email`, and (optional) a client role mapper that exposes `resource_access.<client_id>.roles`.
5. (Optional) Create a client role such as `system_admin` and assign it to any users who should become Mattermost System Admins during login.
6. Copy the generated client secret—you will paste it into the Mattermost plugin settings next.

### 3. Configure the plugin settings

In **System Console → Plugins → Mattermost OIDC**, fill in:

- `Issuer URL`: `https://<keycloak-host>/realms/<realm>` (HTTPS strongly recommended in production).
- `Allow insecure issuer`: leave disabled unless you are on localhost with self-signed certs.
- `Client ID` / `Client Secret`: values from the Keycloak client you created.
- `Redirect URL`: `https://<mattermost-host>/plugins/com.mm.oidc/callback`.
- `Scopes`: typically `openid profile email`; include `roles` if you added the client-role mapper for admin promotion.
- Optional enforcement knobs such as domain allowlists or role-to-admin mapping (see future configuration UI).

Click **Save**, then use the **Start Login** button on the plugin landing page to complete a test round-trip. Successful authentication should provision the user automatically (including system-admin promotion if the Keycloak role is present).

> 💡 Tip: `scripts/dev-up.sh` + `scripts/dev-bootstrap.sh` perform every step above automatically for the Docker-based dev stack. Refer to `docs/DEV_ENV.md` if you prefer automation over manual configuration.

## Development Process

1. **Branching model** – trunk-based flow. Feature branches must include automated tests and documentation updates before merging to `main`.
2. **Code quality gates** – `golangci-lint` for Go, `eslint` + `stylelint` + `tsc --noEmit` for the webapp, `hadolint` for deployment artifacts.
3. **Security posture** – enable Dependabot, Snyk (or similar) scanning, and treat all secrets via environment variables or Mattermost plugin key/value storage with encryption at rest.
4. **Testing pyramid** – fast unit tests, contract tests for the OIDC flow, Cypress-based end-to-end tests against dockerized Mattermost + Keycloak.
5. **Releases** – tagged builds produce signed `.tar.gz` bundles in `build/` and publish GitHub Releases with changelog fragments aggregated from pull requests.

## Repository Layout

```
.
├── build/                 # Versioned plugin bundles and release notes
├── deploy/                # IaC assets (Helm charts, Kubernetes manifests)
├── docs/                  # Architecture, runbooks, threat models
├── scripts/               # Developer automation (lint, package, e2e)
├── server/                # Go backend plugin (Mattermost RPC entrypoints)
├── webapp/                # React/TypeScript webapp bundle
└── .github/workflows/     # Continuous integration pipelines
```

Additional files (created as implementation progresses):

- `Makefile` – canonical entrypoint for linting, testing, packaging, and releasing.
- `go.mod` / `package.json` – language toolchains pinned to secure versions.
- `docs/ARCHITECTURE.md` – deep dive into components, flows, and security (see first draft inside `docs/`).

## Getting Started

- **Prerequisites:** Go 1.22+, Node.js 20 LTS, Yarn 4 (Berry), Docker 25+, Make, jq.
- **Environment:** use `scripts/dev-up.sh` to launch disposable Mattermost + Keycloak instances (see `docs/DEV_ENV.md`). `scripts/dev-down.sh` stops the stack, `scripts/dev-logs.sh` tails logs.
- **Workflow:**
  - `make server-test` exercises the Go unit suite; `make server-build` compiles the linux/amd64 binary consumed by the plugin bundle.
  - `make webapp-build` performs dependency install + type-checked Vite build, `make webapp-test` runs Vitest, and `make webapp-lint` enforces the TypeScript gate.
  - `make package` assembles `build/plugins/mm-oidc.tar.gz` (manifest + server binary + `webapp/dist/main.js`).
  - `make dev-up` / `make dev-down` / `make dev-logs SERVICES="mattermost keycloak"` wrap the docker-compose scripts for local testing.

### Server skeleton

`server/` now exposes:

- A configuration manager that validates issuer/client credentials (HTTPS-only by default) and keeps defaults such as `openid profile email` scopes.
- HTTP routes for `/health`, `/login`, and `/callback`. `/login` performs Authorization Code + PKCE initiation (state/nonce, code verifier storage) while `/callback` now exchanges the authorization code for tokens, verifies the returned `id_token` via JWKS, and provisions/updates a Mattermost user (metadata stored in the plugin KV store for safe subject→user mapping).
- A shared HTTP client with sane timeouts and idempotent router initialization guarded by locks.

### Packaging quickstart

```bash
make package
```

- The Makefile always produces a linux/amd64 binary because the docker-compose stack and release artifacts run on that architecture. Tweak the `GOOS/GOARCH` values directly in the Makefile if you need a one-off build for another platform.
- The manifest (`plugin.json`) references both the server executable and `webapp/dist/main.js`. Ensure `make webapp-build` (which runs `yarn install` + `vite build`) succeeds before packaging.
- After packaging, keep the archive under `build/plugins/` so `scripts/dev-up.sh` can mount it into Mattermost automatically.

If `make` is unavailable, the manual fallback remains:

```bash
cd server
GOOS=linux GOARCH=amd64 go build -o dist/plugin-linux-amd64 ./...
cd ../webapp
corepack yarn install   # first run only
corepack yarn build
cd ..
tar -czvf build/plugins/mm-oidc.tar.gz plugin.json server/dist/plugin-linux-amd64 webapp/dist/main.js
```

### Webapp bundle

`webapp/` hosts a Vite + React + TypeScript bundle (Yarn 4/Berry) that registers a plugin root component. The component surfaces a login CTA inside the Mattermost UI, displays live `/health` metadata, and links to the `/plugins/com.mm.oidc/login` endpoint for manual round-trips.

Key commands:

```bash
cd webapp
corepack yarn install        # once per clone; enables Yarn 4 with node_modules linker
corepack yarn dev            # Vite dev server with HMR
corepack yarn build          # type-check + production bundle -> webapp/dist/main.js
corepack yarn test           # Vitest (jsdom) unit tests
```

The build step emits `webapp/dist/main.js`, which is referenced from `plugin.json` for packaging. `tsc --noEmit` gates the build, so type errors fail CI even without running the dev server.

## Continuous Integration

- Workflow: `.github/workflows/ci.yml`
- Triggers: push, pull_request, or manual `workflow_dispatch`
- Job matrix:
  - `Go Server Tests`: sets up Go 1.22, caches modules, then runs `make server-test`
  - `Webapp Tests`: enables Corepack/Yarn 4, caches the Berry installs, and runs `corepack yarn test`
  - `Dev Stack Bootstrap`: installs both toolchains, runs `scripts/dev-up.sh` (docker compose + `scripts/dev-bootstrap.sh`), ensures every container stays `running` via `docker compose ps --format json`, then tears the stack down

All jobs must succeed before a PR can merge, keeping unit tests green and catching regressions in the docker-compose bootstrap flow. Extend this file when adding linting, integration, or packaging gates.
```

```

## Documentation

- High-level architecture: `docs/ARCHITECTURE.md`
- Local dev/test environment: `docs/DEV_ENV.md`
- Operational runbooks and troubleshooting (to be added under `docs/runbooks/`)
- Security reviews and threat models (to be documented under `docs/threat-model/`)

Contributions must include updates to relevant docs and test suites to keep the repository production-ready.
