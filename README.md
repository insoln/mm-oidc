# Mattermost OIDC Plugin

Mattermost plugin that enables single sign-on with arbitrary OpenID Connect providers (Keycloak by default). The project tracks the latest Mattermost Server (v9.x+) and Keycloak (v25.x+) releases and follows strict security and coding best practices.

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

## Development Process

1. **Branching model** – trunk-based flow. Feature branches must include automated tests and documentation updates before merging to `main`.
2. **Code quality gates** – `golangci-lint` for Go, `eslint` + `stylelint` + `tsc --noEmit` for the webapp, `hadolint` for deployment artifacts.
3. **Security posture** – enable Dependabot, Snyk (or similar) scanning, and treat all secrets via environment variables or Mattermost plugin key/value storage with encryption at rest.
4. **Testing pyramid** – fast unit tests, contract tests for the OIDC flow, Cypress-based end-to-end tests against dockerized Mattermost + Keycloak.
5. **Releases** – tagged builds produce signed `.tar.gz` bundles in `build/` and publish GitHub Releases with changelog fragments aggregated from pull requests.

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
```

```

## Documentation

- High-level architecture: `docs/ARCHITECTURE.md`
- Local dev/test environment: `docs/DEV_ENV.md`
- Operational runbooks and troubleshooting (to be added under `docs/runbooks/`)
- Security reviews and threat models (to be documented under `docs/threat-model/`)

Contributions must include updates to relevant docs and test suites to keep the repository production-ready.
